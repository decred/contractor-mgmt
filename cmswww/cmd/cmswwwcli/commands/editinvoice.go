package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

// RevisionRecord is a record of an invoice revision to the server,
// including both the invoice and the receipt.
type RevisionRecord struct {
	ServerPublicKey  string              `json:"serverpublickey"`
	Revision         v1.EditInvoice      `json:"revision"`
	CensorshipRecord v1.CensorshipRecord `json:"censorshiprecord"`
}

type EditInvoiceCmd struct {
	Args struct {
		Token           string `positional-arg-name:"token"`
		InvoiceFilename string `positional-arg-name:"invoice"`
	} `positional-args:"true" optional:"true"`
}

func (cmd *EditInvoiceCmd) Execute(args []string) error {
	token := cmd.Args.Token
	filename := cmd.Args.InvoiceFilename

	if token == "" || filename == "" {
		return fmt.Errorf("You must supply both an invoice token and the " +
			"filepath to an invoice in proper CSV format.")
	}

	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	id := config.LoggedInUserIdentity
	if id == nil {
		return ErrNotLoggedIn
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

	ei := v1.EditInvoice{
		Token: token,
		File: v1.File{
			MIME:    "text/plain; charset=utf-8",
			Digest:  digest,
			Payload: base64.StdEncoding.EncodeToString(payload),
		},
		PublicKey: hex.EncodeToString(id.Public.Key[:]),
		Signature: hex.EncodeToString(signature[:]),
	}

	var eir v1.EditInvoiceReply
	err = Ctx.Post(v1.RouteEditInvoice, ei, &eir)
	if err != nil {
		return err
	}

	if eir.Invoice.CensorshipRecord.Merkle != digest {
		return fmt.Errorf("Digest returned from server did not match client's"+
			" digest: %v %v", digest, eir.Invoice.CensorshipRecord.Merkle)
	}

	// Store the revision record in case the submitter ever needs it.
	revisionRecord := RevisionRecord{
		ServerPublicKey:  config.ServerPublicKey,
		Revision:         ei,
		CensorshipRecord: eir.Invoice.CensorshipRecord,
	}
	data, err := json.MarshalIndent(revisionRecord, "", "  ")
	if err != nil {
		return err
	}

	revisionRecordFilename, err := config.GetInvoiceSubmissionRecordFilename(
		month, year, eir.Invoice.Version)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(revisionRecordFilename, data, 0400)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoice submitted successfully! The censorship record has"+
			" been stored in %v for your future reference.",
			revisionRecordFilename)
	}

	return nil
}
