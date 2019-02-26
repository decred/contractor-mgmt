package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	pd "github.com/decred/politeia/politeiad/api/v1"
	"github.com/decred/politeia/util"

	v1 "github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

func (c *cmswww) validateInvoiceUnique(user *database.User, month, year uint16) error {
	invoices, _, err := c.db.GetInvoices(database.InvoicesRequest{
		UserID: strconv.FormatUint(user.ID, 10),
		Month:  month,
		Year:   year,
	})
	if err != nil {
		return err
	}

	if len(invoices) > 0 {
		return v1.UserError{
			ErrorCode:    v1.ErrorStatusDuplicateInvoice,
			ErrorContext: []string{invoices[0].Token},
		}
	}

	return nil
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
		UserID:    strconv.FormatUint(user.ID, 10),
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

	// If the files are already loaded for this invoice, return the version
	// from the database; no need to make a request to Politeiad.
	if len(invoice.Files) > 0 {
		idr.Invoice = *invoice
		return &idr, nil
	}

	// Fetch the full record from Politeiad.
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

	invoice.Files = convertRecordFilesToInvoiceFiles(pdReply.Record.Files)

	// Update the database with the files fetched from Politeiad.
	err = c.db.CreateInvoiceFiles(dbInvoice.Token, dbInvoice.Version,
		convertRecordFilesToDatabaseInvoiceFiles(pdReply.Record.Files))
	if err != nil {
		return nil, err
	}

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
	si := req.(*v1.SubmitInvoice)

	err := validateInvoice(si.Signature, si.PublicKey, si.Files,
		int(si.Month), int(si.Year), user)
	if err != nil {
		return nil, err
	}

	err = c.validateInvoiceUnique(user, si.Month, si.Year)
	if err != nil {
		return nil, err
	}

	var nir v1.SubmitInvoiceReply
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	// Assemble metadata record
	ts := time.Now().Unix()
	md, err := json.Marshal(BackendInvoiceMetadata{
		Version:   VersionBackendInvoiceMetadata,
		Month:     si.Month,
		Year:      si.Year,
		Timestamp: ts,
		PublicKey: si.PublicKey,
		Signature: si.Signature,
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
		Files: convertInvoiceFilesToRecordFiles(si.Files),
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

	nir.CensorshipRecord = convertRecordCensorToInvoiceCensor(
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

	err = validateInvoice(ei.Signature, ei.PublicKey, ei.Files,
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
	mdChanges, err := json.Marshal(BackendInvoiceMDChange{
		Version:   VersionBackendInvoiceMDChange,
		Timestamp: ts,
		NewStatus: v1.InvoiceStatusUnreviewedChanges,
	})
	if err != nil {
		return nil, err
	}

	/*
		var delFiles []string
		for _, v := range invRecord.record.Files {
			found := false
			for _, c := range ep.Files {
				if v.Name == c.Name {
					found = true
				}
			}
			if !found {
				delFiles = append(delFiles, v.Name)
			}
		}
	*/

	u := pd.UpdateRecord{
		Token:     ei.Token,
		Challenge: hex.EncodeToString(challenge),
		MDOverwrite: []pd.MetadataStream{{
			ID:      mdStreamGeneral,
			Payload: string(md),
		}},
		MDAppend: []pd.MetadataStream{{
			ID:      mdStreamChanges,
			Payload: string(mdChanges),
		}},
		FilesAdd: convertInvoiceFilesToRecordFiles(ei.Files),
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

	log.Infof("abcde: %v", len(pdUpdateRecordReply.Record.Files))
	dbInvoice, err = c.convertRecordToDatabaseInvoice(
		pdUpdateRecordReply.Record)
	if err != nil {
		return nil, err
	}

	err = c.db.CreateInvoice(dbInvoice)
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
