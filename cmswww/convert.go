package main

import (
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

type BackendInvoiceMDChanges struct {
	Version        uint             `json:"version"`        // Version of the struct
	AdminPublicKey string           `json:"adminpublickey"` // Identity of the administrator
	NewStatus      pd.RecordStatusT `json:"newstatus"`      // NewStatus
	Timestamp      int64            `json:"timestamp"`      // Timestamp of the change
}

func convertInvoiceStatusFromWWW(s v1.InvoiceStatusT) pd.RecordStatusT {
	switch s {
	case v1.InvoiceStatusNotFound:
		return pd.RecordStatusNotFound
	case v1.InvoiceStatusNotReviewed:
		return pd.RecordStatusNotReviewed
	case v1.InvoiceStatusRejected:
		return pd.RecordStatusCensored
	case v1.InvoiceStatusApproved:
		return pd.RecordStatusPublic
	}
	return pd.RecordStatusInvalid
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

// convertInvoiceFromWWW converts a www invoice to a politeiad record.  This
// function should only be used in tests. Note that convertInvoiceFromWWW can not
// emulate MD properly.
func convertInvoiceFromWWW(p v1.InvoiceRecord) pd.Record {
	return pd.Record{
		Status:    convertInvoiceStatusFromWWW(p.Status),
		Timestamp: p.Timestamp,
		Metadata: []pd.MetadataStream{{
			ID:      pd.MetadataStreamsMax + 1, // fail deliberately
			Payload: "invalid payload",
		}},
		Files:            convertInvoiceFileFromWWW(p.File),
		CensorshipRecord: convertInvoiceCensorFromWWW(p.CensorshipRecord),
	}
}

func convertInvoicesFromWWW(p []v1.InvoiceRecord) []pd.Record {
	pr := make([]pd.Record, 0, len(p))
	for _, v := range p {
		pr = append(pr, convertInvoiceFromWWW(v))
	}
	return pr
}

///////////////////////////////
func convertInvoiceStatusFromPD(s pd.RecordStatusT) v1.InvoiceStatusT {
	switch s {
	case pd.RecordStatusNotFound:
		return v1.InvoiceStatusNotFound
	case pd.RecordStatusNotReviewed:
		return v1.InvoiceStatusNotReviewed
	case pd.RecordStatusCensored:
		return v1.InvoiceStatusRejected
	case pd.RecordStatusPublic:
		return v1.InvoiceStatusApproved
	}
	return v1.InvoiceStatusInvalid
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
		invoice.Status = convertInvoiceStatusFromPD(v.NewStatus)
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
		Status:           convertInvoiceStatusFromPD(p.Status),
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
		Status:          convertInvoiceStatusFromPD(p.Status),
		File:            convertRecordFilesToDatabaseInvoiceFile(p.Files),
		Token:           p.CensorshipRecord.Token,
		ServerSignature: p.CensorshipRecord.Signature,
	}
	md := &BackendInvoiceMetadata{}
	for _, m := range p.Metadata {
		switch m.ID {
		case mdStreamGeneral:
			err := json.Unmarshal([]byte(m.Payload), md)
			if err != nil {
				return nil, fmt.Errorf("could not decode metadata '%v' token '%v': %v",
					p.Metadata, p.CensorshipRecord.Token, err)
			}

			dbInvoice.Month = md.Month
			dbInvoice.Year = md.Year
			dbInvoice.Timestamp = md.Timestamp
			dbInvoice.PublicKey = md.PublicKey
			dbInvoice.UserSignature = md.Signature

			dbInvoice.UserID, err = c.db.GetUserIdByPublicKey(md.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("could not get user id from public key %v",
					md.PublicKey)
			}
		case mdStreamChanges:
			f := strings.NewReader(m.Payload)
			d := json.NewDecoder(f)
			for {
				var mdChanges BackendInvoiceMDChanges
				if err := d.Decode(&md); err == io.EOF {
					break
				} else if err != nil {
					return nil, err
				}
				dbInvoice.Changes = append(dbInvoice.Changes,
					convertStreamChangeToDatabaseInvoiceChange(mdChanges))
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

func convertStreamChangeToDatabaseInvoiceChange(mdChanges BackendInvoiceMDChanges) database.InvoiceChange {
	dbInvoiceChange := database.InvoiceChange{}

	dbInvoiceChange.AdminPublicKey = mdChanges.AdminPublicKey
	dbInvoiceChange.NewStatus = convertInvoiceStatusFromPD(mdChanges.NewStatus)
	dbInvoiceChange.Timestamp = mdChanges.Timestamp

	return dbInvoiceChange
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
