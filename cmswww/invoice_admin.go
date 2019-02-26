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

func (c *cmswww) refreshExistingInvoicePayments(dbInvoice *database.Invoice) error {
	for _, dbInvoicePayment := range dbInvoice.Payments {
		if dbInvoicePayment.TxID != "" {
			continue
		}

		dbInvoicePayment.PollExpiry =
			time.Now().Add(pollExpiryDuration).Unix()

		err := c.db.UpdateInvoicePayment(dbInvoice.Token, dbInvoice.Version,
			&dbInvoicePayment)
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
	b, err := base64.StdEncoding.DecodeString(dbInvoice.Files[0].Payload)
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

func (c *cmswww) createInvoicePayment(
	dbInvoice *database.Invoice,
	usdDCRRate float64,
	costUSD uint64,
) (*v1.InvoicePayment, error) {
	invoicePayment := v1.InvoicePayment{
		UserID:   strconv.FormatUint(dbInvoice.UserID, 10),
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
		err = c.db.UpdateInvoicePayment(dbInvoice.Token, dbInvoice.Version,
			dbInvoicePayment)
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

func (c *cmswww) createInvoiceReview(invoice *database.Invoice) (*v1.InvoiceReview, error) {
	invoiceReview := v1.InvoiceReview{
		UserID:    strconv.FormatUint(invoice.UserID, 10),
		Username:  invoice.Username,
		Token:     invoice.Token,
		LineItems: make([]v1.InvoiceReviewLineItem, 0),
	}

	b, err := base64.StdEncoding.DecodeString(invoice.Files[0].Payload)
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

func (c *cmswww) addMDChange(
	invoiceToken string,
	ts int64,
	status v1.InvoiceStatusT,
	adminPublicKey string,
	reason *string,
) (*BackendInvoiceMDChange, error) {
	// Create the change record.
	mdChange := BackendInvoiceMDChange{
		Version:   VersionBackendInvoiceMDChange,
		Timestamp: time.Now().Unix(),
		NewStatus: status,
		Reason:    reason,
	}

	blob, err := json.Marshal(mdChange)
	if err != nil {
		return nil, err
	}

	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	pdCommand := pd.UpdateVettedMetadata{
		Challenge: hex.EncodeToString(challenge),
		Token:     invoiceToken,
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
		return nil, fmt.Errorf("could not unmarshal "+
			"UpdateVettedMetadataReply: %v", err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
	if err != nil {
		return nil, err
	}

	return &mdChange, nil
}

// fetchInvoiceFilesIfNecessary will fetch the invoice files from Politeia
// if they're not set on the invoice. If invoice files needs to be fetched,
// this function will also save them to the database.
func (c *cmswww) fetchInvoiceFilesIfNecessary(dbInvoice *database.Invoice) error {
	if len(dbInvoice.Files) > 0 {
		return nil
	}

	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return err
	}

	responseBody, err := c.rpc(http.MethodPost, pd.GetVettedRoute,
		pd.GetVetted{
			Token:     dbInvoice.Token,
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

	dbInvoice.Files = convertRecordFilesToDatabaseInvoiceFiles(
		pdReply.Record.Files)

	return c.db.CreateInvoiceFiles(dbInvoice.Token, dbInvoice.Version,
		dbInvoice.Files)
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

	log.Infof("Set invoice status, invoice has files: %v", len(dbInvoice.Files))

	adminPublicKey, ok := database.ActiveIdentityString(user.Identities)
	if !ok {
		return nil, fmt.Errorf("invalid admin identity: %v",
			user.ID)
	}

	mdChange, err := c.addMDChange(sis.Token, time.Now().Unix(), sis.Status,
		adminPublicKey, sis.Reason)
	if err != nil {
		return nil, err
	}

	// Update the database with the metadata changes.
	dbInvoiceChange := convertStreamChangeToDatabaseInvoiceChange(*mdChange)
	dbInvoice.Changes = append(dbInvoice.Changes, dbInvoiceChange)

	dbInvoice.Status = dbInvoiceChange.NewStatus
	dbInvoice.StatusChangeReason = dbInvoiceChange.Reason

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
		IncludeFiles: true,
		Page:         -1,
	})
	if err != nil {
		return nil, err
	}

	invoiceReviews := make([]v1.InvoiceReview, 0)
	for _, invoice := range invoices {
		err := c.fetchInvoiceFilesIfNecessary(&invoice)
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

		err = c.fetchInvoiceFilesIfNecessary(invoice)
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

	err = c.fetchInvoiceFilesIfNecessary(invoice)
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

	log.Infof("Num invoice payments: %v", len(dbInvoice.Payments))
	err = c.updateInvoicePayment(dbInvoice, aip.Address, uint64(aip.Amount),
		aip.TxID)
	if err != nil {
		return nil, err
	}

	return &v1.UpdateInvoicePaymentReply{}, nil
}
