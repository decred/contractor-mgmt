package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/decred/dcrtime/merkle"
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

// merkleRoot converts the passed in list of files into SHA256 digests, then
// calculates and returns the merkle root of the digests.
func merkleRoot(files []v1.File) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no invoice files found")
	}

	digests := make([]*[sha256.Size]byte, len(files))
	for i, f := range files {
		d, ok := util.ConvertDigest(f.Digest)
		if !ok {
			return "", fmt.Errorf("could not convert digest")
		}
		digests[i] = &d
	}

	return hex.EncodeToString(merkle.Root(digests)[:]), nil
}

// SignMerkleRoot calculates the merkle root of the passed in list of files,
// signs the merkle root with the passed in identity and returns the signature.
func SignMerkleRoot(files []v1.File, id *identity.FullIdentity) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no invoice files found")
	}
	mr, err := merkleRoot(files)
	if err != nil {
		return "", err
	}
	sig := id.SignMessage([]byte(mr))
	return hex.EncodeToString(sig[:]), nil
}

// VerifyInvoice verifies the integrity of an invoice by verifying the
// invoice's merkle root (if the files are present), the signature,
// and the censorship record signature.
func VerifyInvoice(record v1.InvoiceRecord, serverPubKey string) error {
	// Verify merkle root if the invoice files are present.
	if len(record.Files) > 0 {
		mr, err := merkleRoot(record.Files)
		if err != nil {
			return err
		}
		if mr != record.CensorshipRecord.Merkle {
			return fmt.Errorf("merkle roots do not match; expected %v, got %v",
				mr, record.CensorshipRecord.Merkle)
		}
	}

	// Verify invoice signature.
	pid, err := util.IdentityFromString(record.PublicKey)
	if err != nil {
		return err
	}
	sig, err := util.ConvertSignature(record.Signature)
	if err != nil {
		return err
	}
	if !pid.VerifyMessage([]byte(record.CensorshipRecord.Merkle), sig) {
		return fmt.Errorf("could not verify invoice signature")
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
