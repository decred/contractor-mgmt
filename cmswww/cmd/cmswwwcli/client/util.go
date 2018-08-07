package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/decred/politeia/politeiad/api/v1/identity"
	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

func prettyPrintJSON(v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("Could not marshal JSON: %v\n", err)
	}
	fmt.Fprintf(os.Stdout, "%s\n", b)
	return nil
}

// invoiceSignature signs the file digest with the passed in identity
// and returns the signature.
func invoiceSignature(file v1.File, id *identity.FullIdentity) (string, error) {
	sig := id.SignMessage([]byte(file.Digest))
	return hex.EncodeToString(sig[:]), nil
}

// verifyProposal verifies the integrity of a proposal by verifying the
// proposal's merkle root (if the files are present), the proposal signature,
// and the censorship record signature.
func verifyProposal(record v1.InvoiceRecord, serverPubKey string) error {
	// Verify the file digest.
	if record.File.Digest != record.CensorshipRecord.Merkle {
		return fmt.Errorf("digests do not match")
	}

	// Verify proposal signature.
	pid, err := util.IdentityFromString(record.PublicKey)
	if err != nil {
		return err
	}
	sig, err := util.ConvertSignature(record.Signature)
	if err != nil {
		return err
	}
	if !pid.VerifyMessage([]byte(record.CensorshipRecord.Merkle), sig) {
		return fmt.Errorf("could not verify proposal signature")
	}

	// Verify censorship record signature.
	id, err := util.IdentityFromString(serverPubKey)
	if err != nil {
		return err
	}
	s, err := util.ConvertSignature(record.CensorshipRecord.Signature)
	if err != nil {
		return err
	}
	msg := []byte(record.CensorshipRecord.Merkle + record.CensorshipRecord.Token)
	if !id.VerifyMessage(msg, s) {
		return fmt.Errorf("could not verify censorship record signature")
	}

	return nil
}
