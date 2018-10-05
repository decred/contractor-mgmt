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

	pd "github.com/decred/politeia/politeiad/api/v1"
	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

func validateStatusTransition(dbInvoice *database.Invoice, newStatus v1.InvoiceStatusT) error {
	if dbInvoice.Status == v1.InvoiceStatusNotReviewed ||
		dbInvoice.Status == v1.InvoiceStatusRejected {
		if newStatus == v1.InvoiceStatusApproved ||
			newStatus == v1.InvoiceStatusRejected {
			return nil
		}
	} else if dbInvoice.Status == v1.InvoiceStatusApproved {
		if newStatus == v1.InvoiceStatusPaid {
			return nil
		}
	}

	return v1.UserError{
		ErrorCode: v1.ErrorStatusInvalidInvoiceStatusTransition,
	}
}

func (c *cmswww) createGeneratedInvoice(
	invoice *v1.InvoiceRecord,
	dcrUSDRate float64,
) (*v1.GeneratedInvoice, error) {
	generatedInvoice := v1.GeneratedInvoice{
		UserID:    invoice.UserID,
		Username:  invoice.Username,
		LineItems: make([]v1.GeneratedInvoiceLineItem, 0, 0),
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
		lineItem := v1.GeneratedInvoiceLineItem{}
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
				lineItem.Hours, err = strconv.ParseUint(record[idx], 10, 64)
				if err != nil {
					return nil, err
				}

				generatedInvoice.TotalHours += lineItem.Hours
			case 4:
				lineItem.TotalCost, err = strconv.ParseUint(record[idx], 10, 64)
				if err != nil {
					return nil, err
				}

				generatedInvoice.TotalCostUSD += lineItem.TotalCost
			}
		}

		generatedInvoice.LineItems = append(generatedInvoice.LineItems, lineItem)
	}

	generatedInvoice.TotalCostDCR = float64(generatedInvoice.TotalCostUSD) *
		dcrUSDRate

	// TODO: generate user address from paywall

	return &generatedInvoice, nil
}

// HandleInvoices returns an array of all invoices.
func (c *cmswww) HandleInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	i := req.(*v1.Invoices)

	var statusMap map[v1.InvoiceStatusT]bool
	if i.Status != v1.InvoiceStatusInvalid {
		statusMap[i.Status] = true
	}

	invoices, err := c.getInvoices(database.InvoicesRequest{
		//After:  ui.After,
		//Before: ui.Before,
		Month:     i.Month,
		Year:      i.Year,
		StatusMap: statusMap,
	})
	if err != nil {
		return nil, err
	}

	return &v1.InvoicesReply{
		Invoices: invoices,
	}, nil
}

// HandleGenerateUnpaidInvoices returns an array of all invoices.
func (c *cmswww) HandleGenerateUnpaidInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	gui := req.(*v1.GenerateUnpaidInvoices)

	invoices, err := c.getInvoices(database.InvoicesRequest{
		Month: gui.Month,
		Year:  gui.Year,
		StatusMap: map[v1.InvoiceStatusT]bool{
			v1.InvoiceStatusApproved: true,
		},
	})
	if err != nil {
		return nil, err
	}

	var generatedInvoices []v1.GeneratedInvoice

	for _, invoice := range invoices {
		if invoice.File == nil {
			challenge, err := util.Random(pd.ChallengeSize)
			if err != nil {
				return nil, err
			}

			responseBody, err := c.rpc(http.MethodPost, pd.GetVettedRoute,
				pd.GetVetted{
					Token:     invoice.CensorshipRecord.Token,
					Challenge: hex.EncodeToString(challenge),
				})
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
		}

		generatedInvoice, err := c.createGeneratedInvoice(&invoice,
			gui.DCRUSDRate)
		if err != nil {
			return nil, err
		}

		generatedInvoices = append(generatedInvoices, *generatedInvoice)
	}

	return &v1.GenerateUnpaidInvoicesReply{
		Invoices: generatedInvoices,
	}, nil
}

// HandleMyInvoices returns an array of user's invoices.
func (c *cmswww) HandleMyInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	mi := req.(*v1.MyInvoices)

	statusMap := make(map[v1.InvoiceStatusT]bool)
	if mi.Status != v1.InvoiceStatusInvalid {
		statusMap[mi.Status] = true
	}

	invoices, err := c.getInvoices(database.InvoicesRequest{
		//After:  ui.After,
		//Before: ui.Before,
		UserID:    strconv.FormatUint(user.ID, 10),
		StatusMap: statusMap,
	})
	if err != nil {
		return nil, err
	}

	return &v1.InvoicesReply{
		Invoices: invoices,
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

	err = validateStatusTransition(dbInvoice, sis.Status)
	if err != nil {
		return nil, err
	}

	// Create the change record.
	changes := BackendInvoiceMDChanges{
		Version:   VersionBackendInvoiceMDChanges,
		Timestamp: time.Now().Unix(),
		NewStatus: sis.Status,
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
		return nil, fmt.Errorf("Could not unmarshal SetUnvettedStatusReply: %v",
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
	err = c.db.UpdateInvoice(dbInvoice)
	if err != nil {
		return nil, err
	}

	// Log the action in the admin log.
	c.logAdminInvoiceAction(user, sis.Token,
		fmt.Sprintf("set invoice status to %v",
			v1.InvoiceStatus[sis.Status]))

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

	var (
		isVettedInvoice bool
		route           string
		requestObject   interface{}
	)
	if invoice.Status == v1.InvoiceStatusApproved {
		isVettedInvoice = true
		route = pd.GetVettedRoute
		requestObject = pd.GetVetted{
			Token:     id.Token,
			Challenge: hex.EncodeToString(challenge),
		}
	} else {
		isVettedInvoice = false
		route = pd.GetUnvettedRoute
		requestObject = pd.GetUnvetted{
			Token:     id.Token,
			Challenge: hex.EncodeToString(challenge),
		}
	}

	responseBody, err := c.rpc(http.MethodPost, route, requestObject)
	if err != nil {
		return nil, err
	}

	var response string
	var fullRecord pd.Record
	if isVettedInvoice {
		var pdReply pd.GetVettedReply
		err = json.Unmarshal(responseBody, &pdReply)
		if err != nil {
			return nil, fmt.Errorf("Could not unmarshal "+
				"GetVettedReply: %v", err)
		}

		response = pdReply.Response
		fullRecord = pdReply.Record
	} else {
		var pdReply pd.GetUnvettedReply
		err = json.Unmarshal(responseBody, &pdReply)
		if err != nil {
			return nil, fmt.Errorf("Could not unmarshal "+
				"GetUnvettedReply: %v", err)
		}

		response = pdReply.Response
		fullRecord = pdReply.Record
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge, response)
	if err != nil {
		return nil, err
	}

	invoice.File = convertInvoiceFileFromPD(fullRecord.Files)
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

	err := validateInvoice(ni, user)
	if err != nil {
		return nil, err
	}

	var nir v1.SubmitInvoiceReply
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	// Assemble metdata record
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

	// Create change record
	changes := BackendInvoiceMDChanges{
		Version:   VersionBackendInvoiceMDChanges,
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

	// Add the new proposal to the database.
	err = c.newInventoryRecord(pd.Record{
		Timestamp:        ts,
		CensorshipRecord: pdNewRecordReply.CensorshipRecord,
		Metadata:         pdSetUnvettedStatusReply.Record.Metadata,
		Files:            n.Files,
	})
	if err != nil {
		return nil, err
	}

	nir.CensorshipRecord = convertInvoiceCensorFromPD(
		pdNewRecordReply.CensorshipRecord)
	return &nir, nil
}
