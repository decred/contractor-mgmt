package main

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/decred/politeia/politeiad/api/v1/identity"
	"github.com/decred/politeia/util"
	"github.com/gofrs/uuid"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	validUsername = regexp.MustCompile(createUsernameRegex())
)

func createUsernameRegex() string {
	var buf bytes.Buffer
	buf.WriteString("^[")

	for _, supportedChar := range v1.PolicyUsernameSupportedChars {
		if len(supportedChar) > 1 {
			buf.WriteString(supportedChar)
		} else {
			buf.WriteString(`\` + supportedChar)
		}
	}
	buf.WriteString("]{")
	buf.WriteString(strconv.Itoa(v1.PolicyMinUsernameLength) + ",")
	buf.WriteString(strconv.Itoa(v1.PolicyMaxUsernameLength) + "}$")

	return buf.String()
}

// checkPublicKeyAndSignature validates the public key and signature.
func checkPublicKeyAndSignature(user *database.User, publicKey string, signature string, elements ...string) error {
	id, err := checkPublicKey(user, publicKey)
	if err != nil {
		return err
	}

	return checkSignature(id, signature, elements...)
}

// checkPublicKey compares the supplied public key against the one stored in
// the user database. It will return the active identity if there are no errors.
func checkPublicKey(user *database.User, pk string) ([]byte, error) {
	id, ok := database.ActiveIdentity(user.Identities)
	if !ok {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusNoPublicKey,
		}
	}

	if hex.EncodeToString(id[:]) != pk {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSigningKey,
		}
	}
	return id[:], nil
}

// checkSignature validates an incoming signature against the specified user's pubkey.
func checkSignature(id []byte, signature string, elements ...string) error {
	// Check incoming signature verify(token+string(InvoiceStatus))
	sig, err := util.ConvertSignature(signature)
	if err != nil {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}
	pk, err := identity.PublicIdentityFromBytes(id[:])
	if err != nil {
		return err
	}
	var msg string
	for _, v := range elements {
		msg += v
	}
	if !pk.VerifyMessage([]byte(msg), sig) {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}
	return nil
}

func validateInvoice(
	signature, publicKey, payload string,
	month, year int,
	user *database.User,
) error {
	log.Tracef("validateInvoice")

	// Obtain signature
	sig, err := util.ConvertSignature(signature)
	if err != nil {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}

	// Verify public key
	id, err := checkPublicKey(user, publicKey)
	if err != nil {
		return err
	}

	pk, err := identity.PublicIdentityFromBytes(id[:])
	if err != nil {
		return err
	}

	// Check for the presence of the file.
	if payload == "" {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidInput,
		}
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return err
	}
	digest := util.Digest(data)

	// Validate the string representation of the digest against the signature.
	if !pk.VerifyMessage([]byte(hex.EncodeToString(digest)), sig) {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}

	// Validate that the invoice shows the month and date in a comment.
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	str := fmt.Sprintf("%v %v", v1.PolicyInvoiceCommentChar,
		t.Format("2006-01"))
	if strings.HasPrefix(string(data), str) ||
		strings.Contains(string(data), "\n"+str) {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusMalformedInvoiceFile,
		}
	}

	// Validate that the invoice is CSV-formatted.
	csvReader := csv.NewReader(strings.NewReader(string(data)))
	csvReader.Comma = v1.PolicyInvoiceFieldDelimiterChar
	csvReader.Comment = v1.PolicyInvoiceCommentChar
	csvReader.TrimLeadingSpace = true

	_, err = csvReader.ReadAll()
	if err != nil {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusMalformedInvoiceFile,
		}
	}

	return nil
}

// Invoices should only be viewable by admins and the users who submit them.
func validateUserCanSeeInvoice(invoice *v1.InvoiceRecord, user *database.User) error {
	authorID := invoice.UserID
	if user == nil || (!user.Admin && user.ID.String() != authorID) {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusInvoiceNotFound,
		}
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < v1.PolicyMinPasswordLength {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusMalformedPassword,
		}
	}

	return nil
}

func validatePubkey(publicKey string) ([]byte, error) {
	pk, err := hex.DecodeString(publicKey)
	if err != nil {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidPublicKey,
		}
	}

	var emptyPK [identity.PublicKeySize]byte
	if len(pk) != len(emptyPK) ||
		bytes.Equal(pk, emptyPK[:]) {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidPublicKey,
		}
	}

	return pk, nil
}

func (c *cmswww) validateUsername(username string, userToMatch *database.User) error {
	if len(username) < v1.PolicyMinUsernameLength ||
		len(username) > v1.PolicyMaxUsernameLength {
		log.Tracef("Username not within bounds: %s", username)
		return v1.UserError{
			ErrorCode: v1.ErrorStatusMalformedUsername,
		}
	}

	if !validUsername.MatchString(username) {
		log.Tracef("Username not valid: %s %s", username, validUsername.String())
		return v1.UserError{
			ErrorCode: v1.ErrorStatusMalformedUsername,
		}
	}

	user, err := c.db.GetUserByUsername(username)
	if err != nil && err != database.ErrUserNotFound {
		return err
	}
	if user != nil {
		if userToMatch == nil || user.ID != userToMatch.ID {
			return v1.UserError{
				ErrorCode: v1.ErrorStatusDuplicateUsername,
			}
		}
	}

	return nil
}

func (c *cmswww) validatePubkeyIsUnique(publicKey string, user *database.User) error {
	userID, err := c.db.GetUserIdByPublicKey(publicKey)
	if err != nil && err != database.ErrUserNotFound {
		return err
	}

	if userID == uuid.Nil {
		return nil
	}

	if user != nil && user.ID == userID {
		return nil
	}

	return v1.UserError{
		ErrorCode: v1.ErrorStatusDuplicatePublicKey,
	}
}
