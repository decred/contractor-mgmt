package commands

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type SubmitInvoiceCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" optional:"true"`
	InvoiceFilename string `long:"invoice" optional:"true" description:"Filepath to an invoice CSV"`
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
		return fmt.Errorf("The invoice file (%v) does not exist. Please first "+
			"create it, either manually or through the logwork command.",
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

	csvReader := csv.NewReader(file)
	csvReader.Comma = policy.Invoice.FieldDelimiterChar
	csvReader.Comment = policy.Invoice.CommentChar
	csvReader.TrimLeadingSpace = true

	_, err = csvReader.ReadAll()
	if err != nil {
		file.Close()
		return err
	}
	file.Close()

	file, err = os.Open(filename)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return fmt.Errorf("Unable to read file %v", filename)
	}

	line := strings.TrimSpace(scanner.Text())

	t, err := time.Parse(fmt.Sprintf("%v 2006-01", string(csvReader.Comment)), line)
	if err != nil {
		return fmt.Errorf("CSV should be formatted such that the first line "+
			"is a comment with the month and year in the pattern YYYY-MM: %v", err)
	}

	year = uint16(t.Year())
	month = uint16(t.Month())
	return nil
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

	var filename string
	if cmd.Args.Month != "" && cmd.Args.Year != 0 {
		month, err := ParseMonth(cmd.Args.Month)
		if err != nil {
			return err
		}
		year = cmd.Args.Year

		filename, err = config.GetInvoiceFilename(month, year)
		if err != nil {
			return err
		}
	} else if cmd.InvoiceFilename != "" {
		filename = cmd.InvoiceFilename
	} else {
		return fmt.Errorf("You must supply either a month and year or the " +
			"filepath to an invoice in proper CSV format.")
	}

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
		Year:  year,
		File: v1.File{
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

	filename, err = config.GetInvoiceSubmissionRecordFilename(month, year, "1")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, data, 0400)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		log.Printf("Invoice submitted successfully! The censorship record has"+
			" been stored in %v for your future reference.", filename)
	}

	return nil
}
