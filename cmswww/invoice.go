package main

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/decred/dcrd/dcrutil"
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

func (c *cmswww) refreshExistingInvoicePayments(dbInvoice *database.Invoice) error {
	for _, dbInvoicePayment := range dbInvoice.Payments {
		if dbInvoicePayment.TxID != "" {
			continue
		}

		dbInvoicePayment.PollExpiry =
			time.Now().Add(pollExpiryDuration).Unix()

		err := c.db.UpdateInvoicePayment(&dbInvoicePayment)
		if err != nil {
			return err
		}

		c.addInvoicePaymentForPolling(dbInvoice.Token, &dbInvoicePayment)
	}

	return nil
}

func (c *cmswww) deriveTotalCostFromInvoice(
	dbInvoice *database.Invoice,
	invoicePayment *v1.InvoicePayment,
) error {
	b, err := base64.StdEncoding.DecodeString(dbInvoice.File.Payload)
	if err != nil {
		return err
	}

	csvReader := csv.NewReader(strings.NewReader(string(b)))
	csvReader.Comma = v1.PolicyInvoiceFieldDelimiterChar
	csvReader.Comment = v1.PolicyInvoiceCommentChar
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return err
	}

	for _, record := range records {
		for idx := range v1.InvoiceFields {
			switch idx {
			case 4:
				hours, err := strconv.ParseUint(record[idx], 10, 64)
				if err != nil {
					return err
				}

				invoicePayment.TotalHours += hours
			case 5:
				totalCost, err := strconv.ParseUint(record[idx], 10, 64)
				if err != nil {
					return err
				}

				invoicePayment.TotalCostUSD += totalCost
			}
		}
	}

	return nil
}

func (c *cmswww) createInvoiceReview(invoice *database.Invoice) (*v1.InvoiceReview, error) {
	invoiceReview := v1.InvoiceReview{
		UserID:    invoice.UserID.String(),
		Username:  invoice.Username,
		Token:     invoice.Token,
		LineItems: make([]v1.InvoiceReviewLineItem, 0),
	}

	b, err := base64.StdEncoding.DecodeString(invoice.File.Payload)
	if err != nil {
		return nil, err
	}

	csvReader := csv.NewReader(strings.NewReader(string(b)))
	csvReader.Comma = v1.PolicyInvoiceFieldDelimiterChar
	csvReader.Comment = v1.PolicyInvoiceCommentChar
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		lineItem := v1.InvoiceReviewLineItem{}
		for idx := range v1.InvoiceFields {
			var err error
			switch idx {
			case 0:
				lineItem.Type = record[idx]
			case 1:
				lineItem.Subtype = record[idx]
			case 2:
				lineItem.Description = record[idx]
			case 3:
				lineItem.Proposal = record[idx]
			case 4:
				lineItem.Hours, err = strconv.ParseUint(record[idx], 10, 64)
				if err != nil {
					return nil, err
				}

				invoiceReview.TotalHours += lineItem.Hours
			case 5:
				lineItem.TotalCost, err = strconv.ParseUint(record[idx], 10, 64)
				if err != nil {
					return nil, err
				}

				invoiceReview.TotalCostUSD += lineItem.TotalCost
			}
		}

		invoiceReview.LineItems = append(invoiceReview.LineItems, lineItem)
	}

	return &invoiceReview, nil
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

func (c *cmswww) createInvoicePayment(
	dbInvoice *database.Invoice,
	usdDCRRate float64,
	costUSD uint64,
) (*v1.InvoicePayment, error) {
	invoicePayment := v1.InvoicePayment{
		UserID:   dbInvoice.UserID.String(),
		Username: dbInvoice.Username,
		Token:    dbInvoice.Token,
	}

	var recreatingTotalCostPayment bool
	dbInvoicePayment := &database.InvoicePayment{}
	if costUSD == 0 {
		err := c.deriveTotalCostFromInvoice(dbInvoice, &invoicePayment)
		if err != nil {
			return nil, err
		}

		// If there's already payments on this invoice, determine
		// if one of them is for the total cost.
		for i, payment := range dbInvoice.Payments {
			if payment.IsTotalCost {
				recreatingTotalCostPayment = true
				dbInvoicePayment = &dbInvoice.Payments[i]
				break
			}
		}

		dbInvoicePayment.IsTotalCost = true
	} else {
		invoicePayment.TotalCostUSD = costUSD
	}

	invoicePayment.TotalCostDCR = float64(invoicePayment.TotalCostUSD) / usdDCRRate

	// Generate the user's address.
	user, err := c.db.GetUserById(dbInvoice.UserID)
	if err != nil {
		return nil, err
	}

	// Create or update the invoice payment in the DB.
	address, txNotBefore, err := c.derivePaymentInfo(user)
	if err != nil {
		return nil, err
	}

	amount, err := dcrutil.NewAmount(invoicePayment.TotalCostDCR)
	if err != nil {
		return nil, err
	}

	oldAddress := dbInvoicePayment.Address

	dbInvoicePayment.Address = address
	dbInvoicePayment.TxNotBefore = txNotBefore
	dbInvoicePayment.Amount = uint64(amount)
	dbInvoicePayment.PollExpiry = time.Now().Add(pollExpiryDuration).Unix()
	if !recreatingTotalCostPayment {
		dbInvoice.Payments = append(dbInvoice.Payments, *dbInvoicePayment)
	}

	err = c.updateMDPayments(dbInvoice, false, 0)
	if err != nil {
		return nil, err
	}

	if recreatingTotalCostPayment {
		err = c.db.UpdateInvoicePayment(dbInvoicePayment)
	} else {
		err = c.db.UpdateInvoice(dbInvoice)
	}
	if err != nil {
		return nil, err
	}

	if recreatingTotalCostPayment {
		c.removeInvoicePaymentsFromPolling([]string{oldAddress})
	}
	c.addInvoicePaymentForPolling(dbInvoice.Token, dbInvoicePayment)

	invoicePayment.PaymentAddress = address
	return &invoicePayment, nil
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

func (c *cmswww) fetchInvoiceFileIfNecessary(invoice *database.Invoice) error {
	if invoice.File != nil {
		return nil
	}

	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return err
	}

	responseBody, err := c.rpc(http.MethodPost, pd.GetVettedRoute,
		pd.GetVetted{
			Token:     invoice.Token,
			Challenge: hex.EncodeToString(challenge),
		})
	if err != nil {
		return err
	}

	var pdReply pd.GetVettedReply
	err = json.Unmarshal(responseBody, &pdReply)
	if err != nil {
		return fmt.Errorf("Could not unmarshal "+
			"GetVettedReply: %v", err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
	if err != nil {
		return err
	}

	invoice.File = convertRecordFilesToDatabaseInvoiceFile(pdReply.Record.Files)
	return nil
}

// HandleInvoices returns an array of all invoices.
func (c *cmswww) HandleInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	i := req.(*v1.Invoices)

	statusMap := make(map[v1.InvoiceStatusT]bool)
	if i.Status != v1.InvoiceStatusInvalid {
		statusMap[i.Status] = true
	}

	invoices, numMatches, err := c.getInvoices(database.InvoicesRequest{
		Month:     i.Month,
		Year:      i.Year,
		StatusMap: statusMap,
		Page:      int(i.Page),
	})
	if err != nil {
		return nil, err
	}

	return &v1.InvoicesReply{
		Invoices:     invoices,
		TotalMatches: uint64(numMatches),
	}, nil
}

// HandleReviewInvoices returns a list of all unreviewed invoices.
func (c *cmswww) HandleReviewInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ri := req.(*v1.ReviewInvoices)

	invoices, _, err := c.db.GetInvoices(database.InvoicesRequest{
		Month: ri.Month,
		Year:  ri.Year,
		StatusMap: map[v1.InvoiceStatusT]bool{
			v1.InvoiceStatusNotReviewed:       true,
			v1.InvoiceStatusUnreviewedChanges: true,
		},
		Page: -1,
	})
	if err != nil {
		return nil, err
	}

	var invoiceReviews []v1.InvoiceReview

	for _, invoice := range invoices {
		err := c.fetchInvoiceFileIfNecessary(&invoice)
		if err != nil {
			return nil, err
		}

		invoiceReview, err := c.createInvoiceReview(&invoice)
		if err != nil {
			return nil, err
		}

		invoiceReviews = append(invoiceReviews, *invoiceReview)
	}

	return &v1.ReviewInvoicesReply{
		Invoices: invoiceReviews,
	}, nil
}

// HandlePayInvoices creates new invoice payments and returns their data.
func (c *cmswww) HandlePayInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	pi := req.(*v1.PayInvoices)

	invoices, _, err := c.db.GetInvoices(database.InvoicesRequest{
		Month: pi.Month,
		Year:  pi.Year,
		StatusMap: map[v1.InvoiceStatusT]bool{
			v1.InvoiceStatusApproved: true,
		},
		Page: -1,
	})
	if err != nil {
		return nil, err
	}

	invoicePayments := make([]v1.InvoicePayment, 0)

	for _, inv := range invoices {
		invoice, err := c.db.GetInvoiceByToken(inv.Token)
		if err != nil {
			return nil, err
		}

		err = c.fetchInvoiceFileIfNecessary(invoice)
		if err != nil {
			return nil, err
		}

		err = c.refreshExistingInvoicePayments(invoice)
		if err != nil {
			return nil, err
		}

		invoicePayment, err := c.createInvoicePayment(invoice, pi.USDDCRRate, 0)
		if err != nil {
			return nil, err
		}

		invoicePayments = append(invoicePayments, *invoicePayment)
	}

	return &v1.PayInvoicesReply{
		Invoices: invoicePayments,
	}, nil
}

// HandlePayInvoice creates a new invoice payment and returns it.
func (c *cmswww) HandlePayInvoice(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	pi := req.(*v1.PayInvoice)

	invoice, err := c.db.GetInvoiceByToken(pi.Token)
	if err != nil {
		return nil, err
	}

	err = c.fetchInvoiceFileIfNecessary(invoice)
	if err != nil {
		return nil, err
	}

	err = c.refreshExistingInvoicePayments(invoice)
	if err != nil {
		return nil, err
	}

	invoicePayment, err := c.createInvoicePayment(invoice, pi.USDDCRRate,
		pi.CostUSD)
	if err != nil {
		return nil, err
	}

	return &v1.PayInvoiceReply{
		Invoice: *invoicePayment,
	}, nil
}

// HandleUpdateInvoicePayment updates a payment for an invoice.
func (c *cmswww) HandleUpdateInvoicePayment(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	aip := req.(*v1.UpdateInvoicePayment)

	dbInvoice, err := c.db.GetInvoiceByToken(aip.Token)
	if err != nil {
		if err == database.ErrInvoiceNotFound {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusInvoiceNotFound,
			}
		}

		return nil, err
	}

	err = c.updateInvoicePayment(dbInvoice, aip.Address, uint64(aip.Amount),
		aip.TxID)
	if err != nil {
		return nil, err
	}

	return &v1.UpdateInvoicePaymentReply{}, nil
}

// HandleUserInvoices returns an array of user's invoices.
func (c *cmswww) HandleUserInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ui := req.(*v1.UserInvoices)

	statusMap := make(map[v1.InvoiceStatusT]bool)
	if ui.Status != v1.InvoiceStatusInvalid {
		statusMap[ui.Status] = true
	}

	invoices, numMatches, err := c.getInvoices(database.InvoicesRequest{
		UserID:    user.ID.String(),
		StatusMap: statusMap,
		Page:      int(ui.Page),
	})
	if err != nil {
		return nil, err
	}

	return &v1.InvoicesReply{
		Invoices:     invoices,
		TotalMatches: uint64(numMatches),
	}, nil
}

// HandleSetInvoiceStatus changes the status of an existing invoice
// from unreviewed to either published or rejected.
func (c *cmswww) HandleSetInvoiceStatus(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	sis := req.(*v1.SetInvoiceStatus)

	err := checkPublicKeyAndSignature(user, sis.PublicKey, sis.Signature,
		sis.Token, strconv.FormatUint(uint64(sis.Status), 10))
	if err != nil {
		return nil, err
	}

	dbInvoice, err := c.db.GetInvoiceByToken(sis.Token)
	if err != nil {
		if err == database.ErrInvoiceNotFound {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusInvoiceNotFound,
			}
		}

		return nil, err
	}

	err = validateStatusTransition(dbInvoice, sis.Status, sis.Reason)
	if err != nil {
		return nil, err
	}

	// Create the change record.
	changes := BackendInvoiceMDChange{
		Version:   VersionBackendInvoiceMDChange,
		Timestamp: time.Now().Unix(),
		NewStatus: sis.Status,
		Reason:    sis.Reason,
	}

	var ok bool
	changes.AdminPublicKey, ok = database.ActiveIdentityString(user.Identities)
	if !ok {
		return nil, fmt.Errorf("invalid admin identity: %v",
			user.ID)
	}

	blob, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}

	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	pdCommand := pd.UpdateVettedMetadata{
		Challenge: hex.EncodeToString(challenge),
		Token:     sis.Token,
		MDAppend: []pd.MetadataStream{
			{
				ID:      mdStreamChanges,
				Payload: string(blob),
			},
		},
	}

	responseBody, err := c.rpc(http.MethodPost, pd.UpdateVettedMetadataRoute,
		pdCommand)
	if err != nil {
		return nil, err
	}

	var pdReply pd.UpdateVettedMetadataReply
	err = json.Unmarshal(responseBody, &pdReply)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal UpdateVettedMetadataReply: %v",
			err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
	if err != nil {
		return nil, err
	}

	// Update the database with the metadata changes.
	dbInvoice.Changes = append(dbInvoice.Changes, database.InvoiceChange{
		Timestamp:      changes.Timestamp,
		AdminPublicKey: changes.AdminPublicKey,
		NewStatus:      changes.NewStatus,
	})
	dbInvoice.Status = changes.NewStatus
	if changes.Reason != nil {
		dbInvoice.Changes[len(dbInvoice.Changes)-1].Reason = *changes.Reason
		dbInvoice.StatusChangeReason = *changes.Reason
	} else {
		dbInvoice.StatusChangeReason = ""
	}
	err = c.db.UpdateInvoice(dbInvoice)
	if err != nil {
		return nil, err
	}

	c.fireEvent(EventTypeInvoiceStatusChange,
		EventDataInvoiceStatusChange{
			Invoice:   dbInvoice,
			AdminUser: user,
		},
	)

	// Return the reply.
	sisr := v1.SetInvoiceStatusReply{
		Invoice: *convertDatabaseInvoiceToInvoice(dbInvoice),
	}
	return &sisr, nil
}

// HandleInvoiceDetails tries to fetch the full details of an invoice from
// politeiad.
func (c *cmswww) HandleInvoiceDetails(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	id := req.(*v1.InvoiceDetails)

	var idr v1.InvoiceDetailsReply
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	dbInvoice, err := c.db.GetInvoiceByToken(id.Token)
	if err != nil {
		if err == database.ErrInvoiceNotFound {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusInvoiceNotFound,
			}
		}
		return nil, err
	}

	invoice := convertDatabaseInvoiceToInvoice(dbInvoice)

	err = validateUserCanSeeInvoice(invoice, user)
	if err != nil {
		return nil, err
	}

	responseBody, err := c.rpc(http.MethodPost, pd.GetVettedRoute,
		pd.GetVetted{
			Token:     id.Token,
			Challenge: hex.EncodeToString(challenge),
		},
	)
	if err != nil {
		return nil, err
	}

	var pdReply pd.GetVettedReply
	err = json.Unmarshal(responseBody, &pdReply)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal "+
			"GetVettedReply: %v", err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
	if err != nil {
		return nil, err
	}

	invoice.File = convertInvoiceFileFromPD(pdReply.Record.Files)
	invoice.Username = c.getUsernameByID(invoice.UserID)
	idr.Invoice = *invoice
	return &idr, nil
}

// HandleSubmitInvoice handles the incoming new invoice command.
func (c *cmswww) HandleSubmitInvoice(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ni := req.(*v1.SubmitInvoice)

	err := validateInvoice(ni.Signature, ni.PublicKey, ni.File.Payload,
		int(ni.Month), int(ni.Year), user)
	if err != nil {
		return nil, err
	}

	invoices, _, err := c.db.GetInvoices(database.InvoicesRequest{
		UserID: user.ID.String(),
		Month:  ni.Month,
		Year:   ni.Year,
	})
	if err != nil {
		return nil, err
	}

	if len(invoices) > 0 {
		return nil, v1.UserError{
			ErrorCode:    v1.ErrorStatusDuplicateInvoice,
			ErrorContext: []string{invoices[0].Token},
		}
	}

	var nir v1.SubmitInvoiceReply
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	// Assemble metadata record
	ts := time.Now().Unix()
	md, err := json.Marshal(BackendInvoiceMetadata{
		Month:     ni.Month,
		Year:      ni.Year,
		Version:   VersionBackendInvoiceMetadata,
		Timestamp: ts,
		PublicKey: ni.PublicKey,
		Signature: ni.Signature,
	})
	if err != nil {
		return nil, err
	}

	n := pd.NewRecord{
		Challenge: hex.EncodeToString(challenge),
		Metadata: []pd.MetadataStream{{
			ID:      mdStreamGeneral,
			Payload: string(md),
		}},
		Files: convertInvoiceFileFromWWW(&ni.File),
	}

	var pdNewRecordReply pd.NewRecordReply
	responseBody, err := c.rpc(http.MethodPost, pd.NewRecordRoute, n)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(responseBody, &pdNewRecordReply)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal NewRecordReply: %v",
			err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge,
		pdNewRecordReply.Response)
	if err != nil {
		return nil, err
	}

	// Create change record.
	changes := BackendInvoiceMDChange{
		Version:   VersionBackendInvoiceMDChange,
		Timestamp: time.Now().Unix(),
		NewStatus: v1.InvoiceStatusNotReviewed,
	}

	var pdSetUnvettedStatusReply pd.SetUnvettedStatusReply
	challenge, err = util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	blob, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}

	sus := pd.SetUnvettedStatus{
		Token:     pdNewRecordReply.CensorshipRecord.Token,
		Status:    pd.RecordStatusPublic,
		Challenge: hex.EncodeToString(challenge),
		MDAppend: []pd.MetadataStream{
			{
				ID:      mdStreamChanges,
				Payload: string(blob),
			},
		},
	}

	responseBody, err = c.rpc(http.MethodPost, pd.SetUnvettedStatusRoute, sus)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(responseBody, &pdSetUnvettedStatusReply)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal SetUnvettedStatusReply: %v",
			err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge,
		pdSetUnvettedStatusReply.Response)
	if err != nil {
		return nil, err
	}

	// Add the new invoice to the database.
	err = c.newInventoryRecord(pd.Record{
		Timestamp:        ts,
		CensorshipRecord: pdNewRecordReply.CensorshipRecord,
		Metadata:         pdSetUnvettedStatusReply.Record.Metadata,
		Files:            n.Files,
		Version:          "1",
	})
	if err != nil {
		return nil, err
	}

	nir.CensorshipRecord = convertInvoiceCensorFromPD(
		pdNewRecordReply.CensorshipRecord)
	return &nir, nil
}

// HandleEditInvoice handles the incoming new invoice command.
func (c *cmswww) HandleEditInvoice(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ei := req.(*v1.EditInvoice)

	dbInvoice, err := c.db.GetInvoiceByToken(ei.Token)
	if err != nil {
		return nil, err
	}

	err = validateInvoice(ei.Signature, ei.PublicKey, ei.File.Payload,
		int(dbInvoice.Month), int(dbInvoice.Year), user)
	if err != nil {
		return nil, err
	}
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	// Assemble metadata record
	ts := time.Now().Unix()
	md, err := json.Marshal(BackendInvoiceMetadata{
		Month:     dbInvoice.Month,
		Year:      dbInvoice.Year,
		Version:   VersionBackendInvoiceMetadata,
		Timestamp: ts,
		PublicKey: ei.PublicKey,
		Signature: ei.Signature,
	})
	if err != nil {
		return nil, err
	}

	// Create the change record.
	changes, err := json.Marshal(BackendInvoiceMDChange{
		Version:   VersionBackendInvoiceMDChange,
		Timestamp: ts,
		NewStatus: v1.InvoiceStatusUnreviewedChanges,
	})
	if err != nil {
		return nil, err
	}

	u := pd.UpdateRecord{
		Token:     ei.Token,
		Challenge: hex.EncodeToString(challenge),
		MDOverwrite: []pd.MetadataStream{{
			ID:      mdStreamGeneral,
			Payload: string(md),
		}},
		MDAppend: []pd.MetadataStream{{
			ID:      mdStreamChanges,
			Payload: string(changes),
		}},
		FilesAdd: convertInvoiceFileFromWWW(&ei.File),
	}

	var pdUpdateRecordReply pd.UpdateRecordReply
	responseBody, err := c.rpc(http.MethodPost, pd.UpdateVettedRoute, u)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(responseBody, &pdUpdateRecordReply)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal UpdateRecordReply: %v",
			err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge,
		pdUpdateRecordReply.Response)
	if err != nil {
		return nil, err
	}

	// Update the database with the metadata changes.
	dbInvoice.Changes = append(dbInvoice.Changes, database.InvoiceChange{
		Timestamp: ts,
		NewStatus: v1.InvoiceStatusUnreviewedChanges,
	})
	dbInvoice.Version = pdUpdateRecordReply.Record.Version
	dbInvoice.Status = v1.InvoiceStatusUnreviewedChanges
	err = c.db.UpdateInvoice(dbInvoice)
	if err != nil {
		return nil, err
	}

	c.fireEvent(EventTypeInvoiceStatusChange,
		EventDataInvoiceStatusChange{
			Invoice:   dbInvoice,
			AdminUser: nil,
		},
	)

	reply := &v1.EditInvoiceReply{
		Invoice: *convertDatabaseInvoiceToInvoice(dbInvoice),
	}
	return reply, nil
}
