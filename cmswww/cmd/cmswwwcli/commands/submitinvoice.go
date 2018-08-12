package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	//	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	//	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type SubmitInvoiceCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" optional:"true"`
}

// SubmissionRecord is a record of an invoice submission to the server,
// including both the invoice and the receipt.
type SubmissionRecord struct {
	ServerPublicKey  string              `json:"serverpublickey"`
	Submission       v1.SubmitInvoice    `json:"submission"`
	CensorshipRecord v1.CensorshipRecord `json:"censorshiprecord"`
}

var (
	monthNames = map[string]uint16{
		"january":   1,
		"february":  2,
		"march":     3,
		"april":     4,
		"may":       5,
		"june":      6,
		"july":      7,
		"august":    8,
		"september": 9,
		"october":   10,
		"november":  11,
		"december":  12,
	}
)

func parseMonth(monthStr string) (uint16, error) {
	parsedMonth, err := strconv.ParseUint(monthStr, 10, 16)
	if err == nil {
		return uint16(parsedMonth), nil
	}

	monthStr = strings.ToLower(monthStr)
	month, ok := monthNames[monthStr]
	if ok {
		return month, nil
	}

	for monthName, monthVal := range monthNames {
		if strings.Index(monthName, monthStr) == 0 {
			return monthVal, nil
		}
	}

	return 0, fmt.Errorf("invalid month specified")
}

func (cmd *SubmitInvoiceCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	id := config.LoggedInUserIdentity
	if id == nil {
		return fmt.Errorf("You must be logged in to perform this action.")
	}

	month, err := parseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}

	fname := config.GetInvoiceFilename(month, cmd.Args.Year)
	payload, err := ioutil.ReadFile(fname)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(payload)
	digest := hex.EncodeToString(h.Sum(nil))
	signature := id.SignMessage([]byte(digest))

	ni := v1.SubmitInvoice{
		Month: month,
		Year:  cmd.Args.Year,
		File: v1.File{
			MIME:    "text/plain; charset=utf-8",
			Digest:  digest,
			Payload: base64.StdEncoding.EncodeToString(payload),
		},
		PublicKey: hex.EncodeToString(id.Public.Key[:]),
		Signature: hex.EncodeToString(signature[:]),
	}

	var nir v1.SubmitInvoiceReply
	err = Ctx.Post(v1.RouteSubmitInvoice, ni, &nir)
	if err != nil {
		return err
	}

	if nir.CensorshipRecord.Merkle != digest {
		return fmt.Errorf("Digest returned from server did not match client's"+
			" digest: %v %v", digest, nir.CensorshipRecord.Merkle)
	}

	// Store the submission record in case the submitter ever needs it.
	submissionRecord := SubmissionRecord{
		ServerPublicKey:  config.ServerPublicKey,
		Submission:       ni,
		CensorshipRecord: nir.CensorshipRecord,
	}
	data, err := json.MarshalIndent(submissionRecord, "", "  ")
	if err != nil {
		return err
	}

	fname = config.GetInvoiceSubmissionRecordFilename(month, cmd.Args.Year)
	err = ioutil.WriteFile(fname, data, 0400)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoice submitted successfully! The censorship record has"+
			" been stored in %v for your future reference.", fname)
	}

	return nil
}
