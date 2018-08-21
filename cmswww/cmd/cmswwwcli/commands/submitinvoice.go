package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type SubmitInvoiceCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" required:"true"`
}

// SubmissionRecord is a record of an invoice submission to the server,
// including both the invoice and the receipt.
type SubmissionRecord struct {
	ServerPublicKey  string              `json:"serverpublickey"`
	Submission       v1.SubmitInvoice    `json:"submission"`
	CensorshipRecord v1.CensorshipRecord `json:"censorshiprecord"`
}

func validateInvoiceFile(filename string) error {
	// Verify invoice file exists.
	if !config.FileExists(filename) {
		return fmt.Errorf("The invoice file (%v) does not exist. Please first"+
			"create it, either manually or using the logwork command.",
			filename)
	}

	// Verify the invoice file is formatted correctly according to policy.
	policy, err := fetchPolicy()
	if err != nil {
		return err
	}

	// Verify the invoice file can be read by the CSV parser.
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	csvReader.Comma = policy.Invoice.FieldDelimiterChar
	csvReader.Comment = policy.Invoice.CommentChar
	csvReader.TrimLeadingSpace = true

	_, err = csvReader.ReadAll()
	return err
}

func (cmd *SubmitInvoiceCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	id := config.LoggedInUserIdentity
	if id == nil {
		return ErrNotLoggedIn
	}

	month, err := ParseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}

	filename := config.GetInvoiceFilename(month, cmd.Args.Year)

	err = validateInvoiceFile(filename)
	if err != nil {
		return err
	}

	payload, err := ioutil.ReadFile(filename)
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

	filename = config.GetInvoiceSubmissionRecordFilename(month, cmd.Args.Year)
	err = ioutil.WriteFile(filename, data, 0400)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoice submitted successfully! The censorship record has"+
			" been stored in %v for your future reference.", filename)
	}

	return nil
}
