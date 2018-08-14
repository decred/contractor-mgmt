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

// HandleInvoices returns an array of all invoices.
func (c *cmswww) HandleInvoices(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	i := req.(*v1.Invoices)

	return &v1.InvoicesReply{
		Invoices: c.getInvoices(invoicesRequest{
			//After:  ui.After,
			//Before: ui.Before,
			Month: i.Month,
			Year:  i.Year,
			StatusMap: map[v1.InvoiceStatusT]bool{
				v1.InvoiceStatusNotReviewed: true,
			},
		}),
	}, nil
}

/*
// ProcessSubmitInvoice tries to submit a new invoice to politeiad.
func (c *cmswww) ProcessSubmitInvoice(np v1.SubmitInvoice, user *database.User) (*v1.SubmitInvoiceReply, error) {
	log.Tracef("ProcessSubmitInvoice")

	err := c.validateInvoice(np, user)
	if err != nil {
		return nil, err
	}

	var reply v1.SubmitInvoiceReply
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	// Assemble metdata record
	ts := time.Now().Unix()
	md, err := encodeBackendInvoiceMetadata(BackendInvoiceMetadata{
		Version:   BackendInvoiceMetadataVersion,
		Timestamp: ts,
		PublicKey: np.PublicKey,
		Signature: np.Signature,
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
		Files: convertInvoiceFilesFromWWW(np.Files),
	}

	var pdReply pd.NewRecordReply
	if b.test {
		tokenBytes, err := util.Random(pd.TokenSize)
		if err != nil {
			return nil, err
		}

		pdReply.CensorshipRecord = pd.CensorshipRecord{
			Token: hex.EncodeToString(tokenBytes),
		}

		// Add the new invoice to the cache.
		b.Lock()
		err = b.newInventoryRecord(pd.Record{
			Status:           pd.RecordStatusNotReviewed,
			Timestamp:        ts,
			CensorshipRecord: pdReply.CensorshipRecord,
			Metadata:         n.Metadata,
			Files:            n.Files,
		})
		if err != nil {
			return nil, err
		}
		b.Unlock()
	} else {
		responseBody, err := c.rpc(http.MethodPost,
			pd.NewRecordRoute, n)
		if err != nil {
			return nil, err
		}

		log.Infof("Submitted invoice name: %v", name)
		for k, f := range n.Files {
			log.Infof("%02v: %v %v", k, f.Name, f.Digest)
		}

		err = json.Unmarshal(responseBody, &pdReply)
		if err != nil {
			return nil, fmt.Errorf("Unmarshal SubmitInvoiceReply: %v",
				err)
		}

		// Verify the challenge.
		err = util.VerifyChallenge(c.cfg.Identity, challenge, pdReply.Response)
		if err != nil {
			return nil, err
		}

		// Add the new invoice to the inventory cache.
		b.Lock()
		b.newInventoryRecord(pd.Record{
			Status:           pd.RecordStatusNotReviewed,
			Timestamp:        ts,
			CensorshipRecord: pdReply.CensorshipRecord,
			Metadata:         n.Metadata,
			Files:            n.Files,
		})
		b.Unlock()
	}

	err = b.SpendInvoiceCredit(user, pdReply.CensorshipRecord.Token)
	if err != nil {
		return nil, err
	}

	reply.CensorshipRecord = convertInvoiceCensorFromPD(pdReply.CensorshipRecord)
	return &reply, nil
}

// ProcessSetInvoiceStatus changes the status of an existing invoice
// from unreviewed to either published or rejected.
func (c *cmswww) ProcessSetInvoiceStatus(sps v1.SetInvoiceStatus, user *database.User) (*v1.SetInvoiceStatusReply, error) {
	err := checkPublicKeyAndSignature(user, sps.PublicKey, sps.Signature,
		sps.Token, strconv.FormatUint(uint64(sps.InvoiceStatus), 10))
	if err != nil {
		return nil, err
	}

	// Create change record
	newStatus := convertInvoiceStatusFromWWW(sps.InvoiceStatus)
	r := MDStreamChanges{
		Version:   VersionMDStreamChanges,
		Timestamp: time.Now().Unix(),
		NewStatus: newStatus,
	}

	var reply v1.SetInvoiceStatusReply
	var pdReply pd.SetUnvettedStatusReply
	if b.test {
		pdReply.Record.Status = convertInvoiceStatusFromWWW(sps.InvoiceStatus)
	} else {
		// XXX Expensive to lock but do it for now.
		// Lock is needed to prevent a race into this record and it
		// needs to be updated in the cache.
		b.Lock()
		defer b.Unlock()

		// When not in testnet, block admins
		// from changing the status of their own invoices
		if !c.cfg.TestNet {
			pr, err := b.getInvoice(sps.Token)
			if err != nil {
				return nil, err
			}
			if pr.UserID == strconv.FormatUint(user.ID, 10) {
				return nil, v1.UserError{
					ErrorCode: v1.ErrorStatusReviewerAdminEqualsAuthor,
				}
			}
		}

		challenge, err := util.Random(pd.ChallengeSize)
		if err != nil {
			return nil, err
		}

		var ok bool
		r.AdminPubKey, ok = database.ActiveIdentityString(user.Identities)
		if !ok {
			return nil, fmt.Errorf("invalid admin identity: %v",
				user.ID)
		}

		blob, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}

		sus := pd.SetUnvettedStatus{
			Token:     sps.Token,
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
		b.updateInventoryRecord(pdReply.Record)

		// Log the action in the admin log.
		b.logAdminInvoiceAction(user, sps.Token,
			fmt.Sprintf("set invoice status to %v",
				v1.InvoiceStatus[sps.InvoiceStatus]))
	}

	// Return the reply.
	reply.Invoice = convertInvoiceFromPD(pdReply.Record)

	return &reply, nil
}
*/
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

	c.RLock()
	p, ok := c.inventory[id.Token]
	if !ok {
		c.RUnlock()
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvoiceNotFound,
		}
	}
	c.RUnlock()
	cachedInvoice := convertInvoiceFromInventoryRecord(p, c.userPubkeys)

	err = validateUserCanSeeInvoice(&cachedInvoice, user)
	if err != nil {
		return nil, err
	}

	var (
		isVettedInvoice bool
		route           string
		requestObject   interface{}
	)
	if cachedInvoice.Status == v1.InvoiceStatusApproved {
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

	idr.Invoice = convertInvoiceFromInventoryRecord(&inventoryRecord{
		record:  fullRecord,
		changes: p.changes,
	}, c.userPubkeys)
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
		Version:   BackendInvoiceMetadataVersion,
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

/*
// handleSetInvoiceStatus handles the incoming set invoice status command.
// It's used for either publishing or censoring an invoice.
func (c *cmswww) handleSetInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	// Get the invoice status command.
	log.Tracef("handleSetInvoiceStatus")
	var sps v1.SetInvoiceStatus
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&sps); err != nil {
		RespondWithError(w, r, 0, "handleSetInvoiceStatus: unmarshal", v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidInput,
		})
		return
	}

	user, err := p.GetSessionUser(r)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleSetInvoiceStatus: GetSessionUser %v", err)
		return
	}

	// Set status
	reply, err := p.backend.ProcessSetInvoiceStatus(sps, user)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleSetInvoiceStatus: ProcessSetInvoiceStatus %v", err)
		return
	}

	// Reply with the new invoice status.
	util.RespondWithJSON(w, http.StatusOK, reply)
}

// handleInvoiceDetails handles the incoming invoice details command. It fetches
// the complete details for an existing invoice.
func (c *cmswww) handleInvoiceDetails(w http.ResponseWriter, r *http.Request) {
	// Add the path param to the struct.
	log.Tracef("handleInvoiceDetails")
	pathParams := mux.Vars(r)
	var pd v1.InvoicesDetails
	pd.Token = pathParams["token"]

	user, err := p.GetSessionUser(r)
	if err != nil {
		if err != database.ErrUserNotFound {
			RespondWithError(w, r, 0,
				"handleInvoiceDetails: GetSessionUser %v", err)
			return
		}
	}
	reply, err := p.backend.ProcessInvoiceDetails(pd, user)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleInvoiceDetails: ProcessInvoiceDetails %v", err)
		return
	}

	// Reply with the invoice details.
	util.RespondWithJSON(w, http.StatusOK, reply)
}
*/
