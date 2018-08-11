package main

import (
	"encoding/json"

	www "github.com/decred/contractor-mgmt/cmswww/api/v1"
	pd "github.com/decred/politeia/politeiad/api/v1"
)

type MDStreamChanges struct {
	Version        uint             `json:"version"`        // Version of the struct
	AdminPublicKey string           `json:"adminpublickey"` // Identity of the administrator
	NewStatus      pd.RecordStatusT `json:"newstatus"`      // NewStatus
	Timestamp      int64            `json:"timestamp"`      // Timestamp of the change
}

type BackendInvoiceMetadata struct {
	Version   uint64 `json:"version"` // BackendInvoiceMetadata version
	Month     uint16 `json:"month"`
	Year      uint16 `json:"year"`
	Timestamp int64  `json:"timestamp"` // Last update of invoice
	PublicKey string `json:"publickey"` // Key used for signature.
	Signature string `json:"signature"` // Signature of merkle root
}

func convertInvoiceStatusFromWWW(s www.InvoiceStatusT) pd.RecordStatusT {
	switch s {
	case www.InvoiceStatusNotFound:
		return pd.RecordStatusNotFound
	case www.InvoiceStatusNotReviewed:
		return pd.RecordStatusNotReviewed
	case www.InvoiceStatusRejected:
		return pd.RecordStatusCensored
	case www.InvoiceStatusApproved:
		return pd.RecordStatusPublic
	}
	return pd.RecordStatusInvalid
}

func convertInvoiceFileFromWWW(f *www.File) []pd.File {
	return []pd.File{{
		Name:    "invoice.csv",
		MIME:    f.MIME,
		Digest:  f.Digest,
		Payload: f.Payload,
	}}
}

func convertInvoiceCensorFromWWW(f www.CensorshipRecord) pd.CensorshipRecord {
	return pd.CensorshipRecord{
		Token:     f.Token,
		Merkle:    f.Merkle,
		Signature: f.Signature,
	}
}

// convertInvoiceFromWWW converts a www invoice to a politeiad record.  This
// function should only be used in tests. Note that convertInvoiceFromWWW can not
// emulate MD properly.
func convertInvoiceFromWWW(p www.InvoiceRecord) pd.Record {
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

func convertInvoicesFromWWW(p []www.InvoiceRecord) []pd.Record {
	pr := make([]pd.Record, 0, len(p))
	for _, v := range p {
		pr = append(pr, convertInvoiceFromWWW(v))
	}
	return pr
}

///////////////////////////////
func convertInvoiceStatusFromPD(s pd.RecordStatusT) www.InvoiceStatusT {
	switch s {
	case pd.RecordStatusNotFound:
		return www.InvoiceStatusNotFound
	case pd.RecordStatusNotReviewed:
		return www.InvoiceStatusNotReviewed
	case pd.RecordStatusCensored:
		return www.InvoiceStatusRejected
	case pd.RecordStatusPublic:
		return www.InvoiceStatusApproved
	}
	return www.InvoiceStatusInvalid
}

func convertInvoiceFileFromPD(files []pd.File) *www.File {
	if len(files) == 0 {
		return nil
	}

	return &www.File{
		MIME:    files[0].MIME,
		Digest:  files[0].Digest,
		Payload: files[0].Payload,
	}
}

func convertInvoiceCensorFromPD(f pd.CensorshipRecord) www.CensorshipRecord {
	return www.CensorshipRecord{
		Token:     f.Token,
		Merkle:    f.Merkle,
		Signature: f.Signature,
	}
}

func convertInvoiceFromInventoryRecord(r *inventoryRecord, userPubkeys map[string]string) www.InvoiceRecord {
	invoice := convertInvoiceFromPD(r.record)

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

func convertInvoiceFromPD(p pd.Record) www.InvoiceRecord {
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

	return www.InvoiceRecord{
		Status:           convertInvoiceStatusFromPD(p.Status),
		Timestamp:        md.Timestamp,
		PublicKey:        md.PublicKey,
		Signature:        md.Signature,
		File:             convertInvoiceFileFromPD(p.Files),
		CensorshipRecord: convertInvoiceCensorFromPD(p.CensorshipRecord),
	}
}

func convertErrorStatusFromPD(s int) www.ErrorStatusT {
	switch pd.ErrorStatusT(s) {
	case pd.ErrorStatusInvalidFileDigest:
		return www.ErrorStatusInvalidFileDigest
	case pd.ErrorStatusInvalidBase64:
		return www.ErrorStatusInvalidBase64
	case pd.ErrorStatusInvalidMIMEType:
		return www.ErrorStatusInvalidMIMEType
	case pd.ErrorStatusUnsupportedMIMEType:
		return www.ErrorStatusUnsupportedMIMEType
	case pd.ErrorStatusInvalidRecordStatusTransition:
		return www.ErrorStatusInvalidInvoiceStatusTransition

		// These cases are intentionally omitted because
		// they are indicative of some internal server error,
		// so ErrorStatusInvalid is returned.
		//
		//case pd.ErrorStatusInvalidRequestPayload
		//case pd.ErrorStatusInvalidChallenge
	}
	return www.ErrorStatusInvalid
}
