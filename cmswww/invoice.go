package main

import (
	//"net/http"

	//"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

// HandleUnreviewedInvoices returns an array of all unvetted invoices in reverse order,
// because they're sorted by oldest timestamp first.
func (c *cmswww) HandleUnreviewedInvoices(req interface{}, user *database.User) (interface{}, error) {
	u := req.(v1.UnreviewedInvoices)
	return &v1.UnreviewedInvoicesReply{
		Invoices: c.getInvoices(invoicesRequest{
			//After:  u.After,
			//Before: u.Before,
			Month: u.Month,
			StatusMap: map[v1.InvoiceStatusT]bool{
				v1.InvoiceStatusNotReviewed: true,
			},
		}),
	}, nil
}

/*
// ProcessNewInvoice tries to submit a new invoice to politeiad.
func (c *cmswww) ProcessNewInvoice(np v1.NewInvoice, user *database.User) (*v1.NewInvoiceReply, error) {
	log.Tracef("ProcessNewInvoice")

	err := c.validateInvoice(np, user)
	if err != nil {
		return nil, err
	}

	var reply v1.NewInvoiceReply
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
			return nil, fmt.Errorf("Unmarshal NewInvoiceReply: %v",
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

// ProcessInvoiceDetails tries to fetch the full details of an invoice from politeiad.
func (c *cmswww) ProcessInvoiceDetails(propDetails v1.InvoiceDetails, user *database.User) (*v1.InvoiceDetailsReply, error) {
	log.Debugf("ProcessInvoiceDetails")

	var reply v1.InvoiceDetailsReply
	challenge, err := util.Random(pd.ChallengeSize)
	if err != nil {
		return nil, err
	}

	b.RLock()
	p, ok := b.inventory[propDetails.Token]
	if !ok {
		b.RUnlock()
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvoiceNotFound,
		}
	}
	b.RUnlock()
	cachedInvoice := convertInvoiceFromInventoryRecord(p, c.userPubkeys)

	var isVettedInvoice bool
	var requestObject interface{}
	if cachedInvoice.Status == v1.InvoiceStatusPublic {
		isVettedInvoice = true
		requestObject = pd.GetVetted{
			Token:     propDetails.Token,
			Challenge: hex.EncodeToString(challenge),
		}
	} else {
		isVettedInvoice = false
		requestObject = pd.GetUnvetted{
			Token:     propDetails.Token,
			Challenge: hex.EncodeToString(challenge),
		}
	}

	if b.test {
		reply.Invoice = cachedInvoice
		return &reply, nil
	}

	// The title and files for unvetted invoices should not be viewable by
	// non-admins; only the invoice meta data (status, censorship data, etc)
	// should be publicly viewable.
	isUserAdmin := user != nil && user.Admin
	if !isVettedInvoice && !isUserAdmin {
		reply.Invoice = v1.InvoiceRecord{
			Status:           cachedInvoice.Status,
			Timestamp:        cachedInvoice.Timestamp,
			PublicKey:        cachedInvoice.PublicKey,
			Signature:        cachedInvoice.Signature,
			CensorshipRecord: cachedInvoice.CensorshipRecord,
			NumComments:      cachedInvoice.NumComments,
			UserID:           cachedInvoice.UserID,
			Username:         c.getUsernameByID(cachedInvoice.UserID),
		}

		if user != nil {
			authorID, err := strconv.ParseUint(cachedInvoice.UserID, 10, 64)
			if err != nil {
				return nil, err
			}

			if user.ID == authorID {
				reply.Invoice.Name = cachedInvoice.Name
			}
		}
		return &reply, nil
	}

	var route string
	if isVettedInvoice {
		route = pd.GetVettedRoute
	} else {
		route = pd.GetUnvettedRoute
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

	reply.Invoice = convertInvoiceFromInventoryRecord(&inventoryRecord{
		record:  fullRecord,
		changes: p.changes,
	}, c.userPubkeys)
	reply.Invoice.Username = c.getUsernameByID(reply.Invoice.UserID)
	return &reply, nil
}

// handleNewInvoice handles the incoming new invoice command.
func (c *cmswww) handleNewInvoice(w http.ResponseWriter, r *http.Request) {
	// Get the new invoice command.
	log.Tracef("handleNewInvoice")
	var np v1.NewInvoice
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&np); err != nil {
		RespondWithError(w, r, 0, "handleNewInvoice: unmarshal", v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidInput,
		})
		return
	}

	user, err := p.GetSessionUser(r)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleNewInvoice: GetSessionUser %v", err)
		return
	}

	reply, err := p.backend.ProcessNewInvoice(np, user)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleNewInvoice: ProcessNewInvoice %v", err)
		return
	}

	// Reply with the challenge response and censorship token.
	util.RespondWithJSON(w, http.StatusOK, reply)
}

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
