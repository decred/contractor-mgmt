package commands

import (
	"bufio"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/decred/politeia/politeiad/api/v1/mime"
	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/client"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type SubmitInvoiceCmd struct {
	Args struct {
		Attachments []string `positional-arg-name:"attachmentFilepaths"`
	} `positional-args:"true" optional:"true"`
	InvoiceFilename string `long:"invoice" optional:"true" description:"Filepath to an invoice CSV"`
	Month           string `long:"month" optional:"true" description:"Month to specify a prebuilt invoice"`
	Year            uint16 `long:"year" optional:"true" description:"Year to specify a prebuilt invoice"`
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
	if cmd.Month != "" && cmd.Year != 0 {
		month, err := ParseMonth(cmd.Month)
		if err != nil {
			return err
		}
		year = cmd.Year

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

	// Read attachment files into memory and convert to type File
	files := make([]v1.File, 0, len(cmd.Args.Attachments)+1)

	// Add the invoice file.
	payload, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading invoice file %v: %v", filename, err)
	}

	files = append(files, v1.File{
		Digest:  hex.EncodeToString(util.Digest(payload)),
		Payload: base64.StdEncoding.EncodeToString(payload),
	})

	for _, path := range cmd.Args.Attachments {
		attachment, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading invoice attachment file %v: %v",
				path, err)
		}

		files = append(files, v1.File{
			Name:    filepath.Base(path),
			MIME:    mime.DetectMimeType(attachment),
			Digest:  hex.EncodeToString(util.Digest(attachment)),
			Payload: base64.StdEncoding.EncodeToString(attachment),
		})
	}

	// Compute merkle root and sign it.
	sig, err := client.SignMerkleRoot(files, id)
	if err != nil {
		return fmt.Errorf("SignMerkleRoot: %v", err)
	}

	si := v1.SubmitInvoice{
		Month:     month,
		Year:      year,
		Files:     files,
		PublicKey: hex.EncodeToString(id.Public.Key[:]),
		Signature: sig,
	}

	var sir v1.SubmitInvoiceReply
	err = Ctx.Post(v1.RouteSubmitInvoice, si, &sir)
	if err != nil {
		return err
	}

	ir := v1.InvoiceRecord{
		Files:            si.Files,
		PublicKey:        si.PublicKey,
		Signature:        si.Signature,
		CensorshipRecord: sir.CensorshipRecord,
	}
	err = client.VerifyInvoice(ir, config.ServerPublicKey)
	if err != nil {
		return err
	}

	// Store the submission record in case the submitter ever needs it.
	submissionRecord := SubmissionRecord{
		ServerPublicKey:  config.ServerPublicKey,
		Submission:       si,
		CensorshipRecord: sir.CensorshipRecord,
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
		fmt.Printf("Invoice submitted successfully! The censorship record has"+
			" been stored in %v for your future reference.", filename)
	}

	return nil
}
