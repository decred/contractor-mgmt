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

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

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

	return &v1.InvoicesReply{
		/*
			Invoices: c.getInvoices(invoicesRequest{
				//After:  ui.After,
				//Before: ui.Before,
				Month:     i.Month,
				Year:      i.Year,
				StatusMap: statusMap,
			}),
		*/
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

	// Create change record
	newStatus := convertInvoiceStatusFromWWW(sis.Status)
	changes := BackendInvoiceMDChanges{
		Version:   VersionBackendInvoiceMDChanges,
		Timestamp: time.Now().Unix(),
		NewStatus: newStatus,
	}

	var pdReply pd.SetUnvettedStatusReply

	// XXX Expensive to lock but do it for now.
	// Lock is needed to prevent a race into this record and it
	// needs to be updated in the cache.
	c.Lock()
	defer c.Unlock()

	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
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

	sus := pd.SetUnvettedStatus{
		Token:     sis.Token,
		Status:    newStatus,
		Challenge: hex.EncodeToString(challenge),
		MDAppend: []pd.MetadataStream{
			{
				ID:      mdStreamChanges,
				Payload: string(blob),
			},
		},
	}

	responseBody, err := c.rpc(http.MethodPost,
		pd.SetUnvettedStatusRoute, sus)
	if err != nil {
		return nil, err
	}

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

	// Update the inventory with the metadata changes.
	//c.updateInventoryRecord(pdReply.Record)

	// Log the action in the admin log.
	c.logAdminInvoiceAction(user, sis.Token,
		fmt.Sprintf("set invoice status to %v",
			v1.InvoiceStatus[sis.Status]))

	// Return the reply.
	sisr := v1.SetInvoiceStatusReply{
		//Invoice: convertInvoiceFromPD(pdReply.Record),
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

	idr.Invoice = convertRecordToInvoice(fullRecord)
	idr.Invoice.Username = c.getUsernameByID(idr.Invoice.UserID)
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

	var pdReply pd.NewRecordReply
	responseBody, err := c.rpc(http.MethodPost,
		pd.NewRecordRoute, n)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(responseBody, &pdReply)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal NewRecordReply: %v",
			err)
	}

	// Verify the challenge.
	err = util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
	if err != nil {
		return nil, err
	}

	// Add the new proposal to the inventory cache.
	c.Lock()
	c.newInventoryRecord(pd.Record{
		Status:           pd.RecordStatusNotReviewed,
		Timestamp:        ts,
		CensorshipRecord: pdReply.CensorshipRecord,
		Metadata:         n.Metadata,
		Files:            n.Files,
	})
	c.Unlock()

	nir.CensorshipRecord = convertInvoiceCensorFromPD(pdReply.CensorshipRecord)
	return &nir, nil
}
