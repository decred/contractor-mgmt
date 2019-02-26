package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pd "github.com/decred/politeia/politeiad/api/v1"
	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	validStatusTransitions = map[v1.InvoiceStatusT][]v1.InvoiceStatusT{
		v1.InvoiceStatusNotReviewed: {
			v1.InvoiceStatusApproved,
			v1.InvoiceStatusRejected,
		},
		v1.InvoiceStatusRejected: {
			v1.InvoiceStatusApproved,
			v1.InvoiceStatusUnreviewedChanges,
		},
		v1.InvoiceStatusUnreviewedChanges: {
			v1.InvoiceStatusApproved,
			v1.InvoiceStatusRejected,
		},
		v1.InvoiceStatusApproved: {
			v1.InvoiceStatusPaid,
		},
	}
)

func statusInSlice(arr []v1.InvoiceStatusT, status v1.InvoiceStatusT) bool {
	for _, s := range arr {
		if status == s {
			return true
		}
	}

	return false
}

func validateStatusTransition(
	dbInvoice *database.Invoice,
	newStatus v1.InvoiceStatusT,
	reason *string,
) error {
	validStatuses, ok := validStatusTransitions[dbInvoice.Status]
	if !ok {
		log.Errorf("status not supported: %v", dbInvoice.Status)
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidInvoiceStatusTransition,
		}
	}

	if !statusInSlice(validStatuses, newStatus) {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidInvoiceStatusTransition,
		}
	}

	if newStatus == v1.InvoiceStatusRejected && reason == nil {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusReasonNotProvided,
		}
	}

	return nil
}

func (c *cmswww) updateMDPayments(
	dbInvoice *database.Invoice,
	updatingInvoicePayment bool,
	ts int64,
) error {
	// Create the payments metadata record.
	mdPayments, err := convertDatabaseInvoicePaymentsToStreamPayments(dbInvoice)
	if err != nil {
		return err
	}

	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return fmt.Errorf("could not create challenge: %v", err)
	}

	pdCommand := pd.UpdateVettedMetadata{
		Challenge: hex.EncodeToString(challenge),
		Token:     dbInvoice.Token,
		MDOverwrite: []pd.MetadataStream{
			{
				ID:      mdStreamPayments,
				Payload: mdPayments,
			},
		},
	}

	// Create the change metadata record if an existing invoice payment
	// is being updated.
	if updatingInvoicePayment && dbInvoice.Status != v1.InvoiceStatusPaid {
		mdChange, err := json.Marshal(BackendInvoiceMDChange{
			Version:   VersionBackendInvoiceMDChange,
			Timestamp: ts,
			NewStatus: v1.InvoiceStatusPaid,
		})
		if err != nil {
			return fmt.Errorf("cannot marshal backend change: %v", err)
		}

		pdCommand.MDAppend = []pd.MetadataStream{
			{
				ID:      mdStreamChanges,
				Payload: string(mdChange),
			},
		}
	}

	responseBody, err := c.rpc(http.MethodPost, pd.UpdateVettedMetadataRoute,
		pdCommand)
	if err != nil {
		return err
	}

	var pdReply pd.UpdateVettedMetadataReply
	err = json.Unmarshal(responseBody, &pdReply)
	if err != nil {
		return fmt.Errorf("Could not unmarshal UpdateVettedMetadataReply: %v",
			err)
	}

	// Verify the challenge.
	return util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
}

func (c *cmswww) updateInvoicePayment(
	dbInvoice *database.Invoice,
	address string,
	amount uint64,
	txID string,
) error {
	var dbInvoicePayment *database.InvoicePayment
	for idx, payment := range dbInvoice.Payments {
		if payment.Amount == amount && payment.Address == address {
			dbInvoice.Payments[idx].TxID = txID
			dbInvoicePayment = &payment
			break
		}
	}

	if dbInvoicePayment == nil {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvoicePaymentNotFound,
		}
	}

	// Update the Politeia record.
	ts := time.Now().Unix()
	err := c.updateMDPayments(dbInvoice, true, ts)
	if err != nil {
		return err
	}

	// Update the invoice in the database.
	if dbInvoice.Status != v1.InvoiceStatusPaid {
		// Update the status in the database if necessary.
		dbInvoice.Status = v1.InvoiceStatusPaid
		dbInvoice.Changes = append(dbInvoice.Changes, database.InvoiceChange{
			Timestamp: ts,
			NewStatus: v1.InvoiceStatusPaid,
		})
	}
	err = c.db.UpdateInvoice(dbInvoice)
	if err != nil {
		return fmt.Errorf("cannot update invoice with token %v: %v",
			dbInvoice.Token, err)
	}

	return nil
}
