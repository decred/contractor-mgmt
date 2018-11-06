package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	v1 "github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
	pd "github.com/decred/politeia/politeiad/api/v1"
)

type BackendInvoiceMetadata struct {
	Version   uint64 `json:"version"` // BackendInvoiceMetadata version
	Month     uint16 `json:"month"`
	Year      uint16 `json:"year"`
	Timestamp int64  `json:"timestamp"` // Last update of invoice
	PublicKey string `json:"publickey"` // Key used for signature.
	Signature string `json:"signature"` // Signature of merkle root
}

type BackendInvoiceMDChange struct {
	Version        uint              `json:"version"`        // Version of the struct
	AdminPublicKey string            `json:"adminpublickey"` // Identity of the administrator
	NewStatus      v1.InvoiceStatusT `json:"newstatus"`      // Status
	Timestamp      int64             `json:"timestamp"`      // Timestamp of the change
}

type BackendInvoiceMDPayment struct {
	Version     uint   `json:"version"`     // Version of the struct
	Address     string `json:"address"`     // Payment address
	Amount      uint64 `json:"amount"`      // Payment amount in atoms
	TxNotBefore int64  `json:"txnotbefore"` // Minimum UNIX time for the transaction to be accepted as payment
	TxID        string `json:"txid"`        // Transaction ID of the actual payment
}

func convertDatabaseUserToUser(user *database.User) v1.User {
	return v1.User{
		ID:                strconv.FormatUint(user.ID, 10),
		Email:             user.Email,
		Username:          user.Username,
		Name:              user.Name,
		Location:          user.Location,
		ExtendedPublicKey: user.ExtendedPublicKey,
		Admin:             user.Admin,
		RegisterVerificationToken:        user.RegisterVerificationToken,
		RegisterVerificationExpiry:       user.RegisterVerificationExpiry,
		UpdateIdentityVerificationToken:  user.UpdateIdentityVerificationToken,
		UpdateIdentityVerificationExpiry: user.UpdateIdentityVerificationExpiry,
		LastLogin:                        user.LastLogin,
		FailedLoginAttempts:              user.FailedLoginAttempts,
		Locked:                           IsUserLocked(user.FailedLoginAttempts),
		Identities:                       convertDatabaseIdentitiesToIdentities(user.Identities),
	}
}

func convertDatabaseIdentitiesToIdentities(dbIdentities []database.Identity) []v1.UserIdentity {
	identities := make([]v1.UserIdentity, 0, len(dbIdentities))
	for _, dbIdentity := range dbIdentities {
		identities = append(identities, convertDatabaseIdentityToIdentity(dbIdentity))
	}
	return identities
}

func convertDatabaseIdentityToIdentity(dbIdentity database.Identity) v1.UserIdentity {
	return v1.UserIdentity{
		PublicKey: hex.EncodeToString(dbIdentity.Key[:]),
		Active:    dbIdentity.IsActive(),
	}
}

func convertInvoiceFileFromWWW(f *v1.File) []pd.File {
	return []pd.File{{
		Name:    "invoice.csv",
		MIME:    f.MIME,
		Digest:  f.Digest,
		Payload: f.Payload,
	}}
}

func convertInvoiceCensorFromWWW(f v1.CensorshipRecord) pd.CensorshipRecord {
	return pd.CensorshipRecord{
		Token:     f.Token,
		Merkle:    f.Merkle,
		Signature: f.Signature,
	}
}

func convertInvoiceFileFromPD(files []pd.File) *v1.File {
	if len(files) == 0 {
		return nil
	}

	return &v1.File{
		MIME:    files[0].MIME,
		Digest:  files[0].Digest,
		Payload: files[0].Payload,
	}
}

func convertRecordFilesToDatabaseInvoiceFile(files []pd.File) *database.File {
	if len(files) == 0 {
		return nil
	}

	return &database.File{
		MIME:    files[0].MIME,
		Digest:  files[0].Digest,
		Payload: files[0].Payload,
	}
}

func convertInvoiceCensorFromPD(f pd.CensorshipRecord) v1.CensorshipRecord {
	return v1.CensorshipRecord{
		Token:     f.Token,
		Merkle:    f.Merkle,
		Signature: f.Signature,
	}
}

func convertInvoiceFromInventoryRecord(r *inventoryRecord, userPubkeys map[string]string) v1.InvoiceRecord {
	invoice := convertRecordToInvoice(r.record)

	// Set the most up-to-date status.
	for _, v := range r.changes {
		invoice.Status = v.NewStatus
	}

	// Set the user id.
	var ok bool
	invoice.UserID, ok = userPubkeys[invoice.PublicKey]
	if !ok {
		log.Errorf("user not found for public key %v, for invoice %v",
			invoice.PublicKey, invoice.CensorshipRecord.Token)
	}

	return invoice
}

func convertRecordToInvoice(p pd.Record) v1.InvoiceRecord {
	md := &BackendInvoiceMetadata{}
	for _, v := range p.Metadata {
		if v.ID != mdStreamGeneral {
			continue
		}
		err := json.Unmarshal([]byte(v.Payload), md)
		if err != nil {
			log.Errorf("could not decode metadata '%v' token '%v': %v",
				p.Metadata, p.CensorshipRecord.Token, err)
			break
		}
	}

	return v1.InvoiceRecord{
		Timestamp:        md.Timestamp,
		Month:            md.Month,
		Year:             md.Year,
		PublicKey:        md.PublicKey,
		Signature:        md.Signature,
		File:             convertInvoiceFileFromPD(p.Files),
		CensorshipRecord: convertInvoiceCensorFromPD(p.CensorshipRecord),
	}
}

func (c *cmswww) convertRecordToDatabaseInvoice(p pd.Record) (*database.Invoice, error) {
	dbInvoice := database.Invoice{
		File:            convertRecordFilesToDatabaseInvoiceFile(p.Files),
		Token:           p.CensorshipRecord.Token,
		ServerSignature: p.CensorshipRecord.Signature,
	}
	for _, m := range p.Metadata {
		switch m.ID {
		case mdStreamGeneral:
			var mdGeneral BackendInvoiceMetadata
			err := json.Unmarshal([]byte(m.Payload), &mdGeneral)
			if err != nil {
				return nil, fmt.Errorf("could not decode metadata '%v' token '%v': %v",
					p.Metadata, p.CensorshipRecord.Token, err)
			}

			dbInvoice.Month = mdGeneral.Month
			dbInvoice.Year = mdGeneral.Year
			dbInvoice.Timestamp = mdGeneral.Timestamp
			dbInvoice.PublicKey = mdGeneral.PublicKey
			dbInvoice.UserSignature = mdGeneral.Signature

			dbInvoice.UserID, err = c.db.GetUserIdByPublicKey(mdGeneral.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("could not get user id from public key %v",
					mdGeneral.PublicKey)
			}
		case mdStreamChanges:
			f := strings.NewReader(m.Payload)
			d := json.NewDecoder(f)
			for {
				var mdChange BackendInvoiceMDChange
				if err := d.Decode(&mdChange); err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}

				dbInvoice.Changes = append(dbInvoice.Changes,
					convertStreamChangeToDatabaseInvoiceChange(mdChange))
				dbInvoice.Status = mdChange.NewStatus
			}
		case mdStreamPayments:
			f := strings.NewReader(m.Payload)
			d := json.NewDecoder(f)
			for {
				var mdPayment BackendInvoiceMDPayment
				if err := d.Decode(&mdPayment); err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}

				dbInvoice.Payments = append(dbInvoice.Payments,
					convertStreamPaymentToDatabaseInvoicePayment(mdPayment))
			}
		default:
			// Log error but proceed
			log.Errorf("initializeInventory: invalid "+
				"metadata stream ID %v token %v",
				m.ID, p.CensorshipRecord.Token)
		}
	}

	return &dbInvoice, nil
}

func convertStreamChangeToDatabaseInvoiceChange(mdChange BackendInvoiceMDChange) database.InvoiceChange {
	dbInvoiceChange := database.InvoiceChange{}

	dbInvoiceChange.AdminPublicKey = mdChange.AdminPublicKey
	dbInvoiceChange.NewStatus = mdChange.NewStatus
	dbInvoiceChange.Timestamp = mdChange.Timestamp

	return dbInvoiceChange
}

func convertStreamPaymentToDatabaseInvoicePayment(mdPayment BackendInvoiceMDPayment) database.InvoicePayment {
	dbInvoicePayment := database.InvoicePayment{}

	dbInvoicePayment.Address = mdPayment.Address
	dbInvoicePayment.Amount = mdPayment.Amount
	dbInvoicePayment.TxNotBefore = mdPayment.TxNotBefore
	dbInvoicePayment.TxID = mdPayment.TxID

	return dbInvoicePayment
}

func convertDatabaseInvoiceToInvoice(dbInvoice *database.Invoice) *v1.InvoiceRecord {
	invoice := v1.InvoiceRecord{}

	invoice.Status = dbInvoice.Status
	invoice.Timestamp = dbInvoice.Timestamp
	invoice.Month = dbInvoice.Month
	invoice.Year = dbInvoice.Year
	invoice.UserID = strconv.FormatUint(dbInvoice.UserID, 10)
	invoice.Username = dbInvoice.Username
	invoice.PublicKey = dbInvoice.PublicKey
	invoice.Signature = dbInvoice.UserSignature
	if dbInvoice.File != nil {
		invoice.File = &v1.File{
			MIME:    dbInvoice.File.MIME,
			Digest:  dbInvoice.File.Digest,
			Payload: dbInvoice.File.Payload,
		}
	}
	invoice.CensorshipRecord = v1.CensorshipRecord{
		Token: dbInvoice.Token,
		//Merkle:    dbInvoice.File.Digest,
		Signature: dbInvoice.ServerSignature,
	}

	// TODO: clean up, merkle should always be set
	if dbInvoice.File != nil {
		invoice.CensorshipRecord.Merkle = dbInvoice.File.Digest
	}

	return &invoice
}

func convertDatabaseInvoicesToInvoices(dbInvoices []database.Invoice) []v1.InvoiceRecord {
	invoices := make([]v1.InvoiceRecord, 0, len(dbInvoices))
	for _, dbInvoice := range dbInvoices {
		invoices = append(invoices, *convertDatabaseInvoiceToInvoice(&dbInvoice))
	}
	return invoices
}

func convertErrorStatusFromPD(s int) v1.ErrorStatusT {
	switch pd.ErrorStatusT(s) {
	case pd.ErrorStatusInvalidFileDigest:
		return v1.ErrorStatusInvalidFileDigest
	case pd.ErrorStatusInvalidBase64:
		return v1.ErrorStatusInvalidBase64
	case pd.ErrorStatusInvalidMIMEType:
		return v1.ErrorStatusInvalidMIMEType
	case pd.ErrorStatusUnsupportedMIMEType:
		return v1.ErrorStatusUnsupportedMIMEType
	case pd.ErrorStatusInvalidRecordStatusTransition:
		return v1.ErrorStatusInvalidInvoiceStatusTransition

		// These cases are intentionally omitted because
		// they are indicative of some internal server error,
		// so ErrorStatusInvalid is returned.
		//
		//case pd.ErrorStatusInvalidRequestPayload
		//case pd.ErrorStatusInvalidChallenge
	}
	return v1.ErrorStatusInvalid
}
