package main

import (
	"bytes"
	"encoding/hex"
	"github.com/decred/politeia/politeiad/api/v1/identity"
	"net/http"
	"strconv"
	"time"

	"github.com/decred/politeia/util"
	"golang.org/x/crypto/bcrypt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

// IsUserLocked returns true if the number of failed login attempts exceeds
// the threshold for locking a user account.
func IsUserLocked(failedLoginAttempts uint64) bool {
	return failedLoginAttempts >= v1.LoginAttemptsToLockUser
}

// hashPassword hashes the given password string with the default bcrypt cost
// or the minimum cost if the test flag is set to speed up running tests.
func hashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password),
		bcrypt.DefaultCost)
}

func (c *cmswww) generateVerificationTokenAndExpiry() ([]byte, int64, error) {
	token, err := util.Random(v1.VerificationTokenSize)
	if err != nil {
		return nil, 0, err
	}

	expiry := time.Now().Add(v1.VerificationExpiryTime).Unix()

	return token, expiry, nil
}

// getUsernameByID returns the username given its id. If the id is invalid,
// it returns an empty string.
func (c *cmswww) getUsernameByID(idStr string) string {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return ""
	}

	user, err := c.db.UserGetById(id)
	if err != nil {
		return ""
	}

	return user.Username
}

// Performs a user lookup using id, email, or username (in that order). Email
// lookup is only possible if the user requesting the information is an admin.
func (c *cmswww) findUser(idStr, email, username string, isAdmin bool) (*database.User, error) {
	var (
		user *database.User
		err  error
	)

	if idStr != "" {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err == nil {
			user, err = c.db.UserGetById(id)
			if err != nil && err != database.ErrUserNotFound {
				return nil, err
			}
		}
	}

	if user == nil && isAdmin && email != "" {
		user, err = c.db.UserGetByEmail(email)
		if err != nil && err != database.ErrUserNotFound {
			return nil, err
		}
	}

	if user == nil && username != "" {
		user, err = c.db.UserGetByUsername(username)
		if err != nil && err != database.ErrUserNotFound {
			return nil, err
		}
	}

	if user != nil {
		return user, nil
	}

	return nil, v1.UserError{
		ErrorCode: v1.ErrorStatusUserNotFound,
	}
}

// HandleRegister verifies the token generated for a recently created
// user.  It ensures that the token matches with the input and that the token
// hasn't expired.  On success it returns database user record.
func (c *cmswww) HandleRegister(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	nu := req.(*v1.Register)

	// Check that the user already exists.
	user, err := c.db.UserGetByEmail(nu.Email)
	if err != nil {
		if err == database.ErrUserNotFound {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
			}
		}
		return nil, err
	}

	// Decode the verification token.
	token, err := hex.DecodeString(nu.VerificationToken)
	if err != nil {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the verification token matches.
	if !bytes.Equal(token, user.RegisterVerificationToken) {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the token hasn't expired.
	if time.Now().Unix() > user.RegisterVerificationExpiry {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenExpired,
		}
	}

	// Ensure we got a proper pubkey.
	pk, err := validatePubkey(nu.PublicKey)
	if err != nil {
		return nil, err
	}

	// Validate the username.
	err = c.validateUsername(nu.Username, nil)
	if err != nil {
		return nil, err
	}

	// Validate the password.
	err = validatePassword(nu.Password)
	if err != nil {
		return nil, err
	}

	// Validate that the pubkey isn't already taken.
	err = c.validatePubkeyIsUnique(nu.PublicKey, user)
	if err != nil {
		return nil, err
	}

	// Validate the signature against the public key.
	sig, err := util.ConvertSignature(nu.Signature)
	if err != nil {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}

	pi, err := identity.PublicIdentityFromBytes(pk)
	if err != nil {
		return nil, err
	}

	if !pi.VerifyMessage([]byte(nu.VerificationToken), sig) {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}

	// Hash the user's password.
	hashedPassword, err := hashPassword(nu.Password)
	if err != nil {
		return nil, err
	}

	// Associate the user id with the new public key.
	c.SetUserPubkeyAssociaton(user, nu.PublicKey)

	// Update the user in the db.
	user.RegisterVerificationToken = nil
	user.RegisterVerificationExpiry = 0
	user.HashedPassword = hashedPassword
	user.Username = nu.Username
	user.Identities = []database.Identity{{
		Activated: time.Now().Unix(),
	}}
	copy(user.Identities[0].Key[:], pk)

	err = c.db.UserUpdate(user)
	return &v1.RegisterReply{}, err
}

func (c *cmswww) HandleNewIdentity(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ni := req.(*v1.NewIdentity)

	var token []byte
	var expiry int64

	// Ensure we got a proper pubkey.
	pk, err := validatePubkey(ni.PublicKey)
	if err != nil {
		return nil, err
	}

	// Validate that the pubkey isn't already taken.
	err = c.validatePubkeyIsUnique(ni.PublicKey, user)
	if err != nil {
		return nil, err
	}

	// Check if the verification token hasn't expired yet.
	if user.UpdateIdentityVerificationToken != nil {
		if user.UpdateIdentityVerificationExpiry > time.Now().Unix() {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusVerificationTokenUnexpired,
				ErrorContext: []string{
					strconv.FormatInt(user.UpdateIdentityVerificationExpiry, 10),
				},
			}
		}
	}

	// Generate a new verification token and expiry.
	token, expiry, err = c.generateVerificationTokenAndExpiry()
	if err != nil {
		return nil, err
	}

	// Add the updated user information to the db.
	user.UpdateIdentityVerificationToken = token
	user.UpdateIdentityVerificationExpiry = expiry

	identity := database.Identity{}
	copy(identity.Key[:], pk)
	user.Identities = append(user.Identities, identity)

	err = c.db.UserUpdate(user)
	if err != nil {
		return nil, err
	}

	// This is conditional on the email server being setup.
	err = c.emailUpdateIdentityVerificationLink(user.Email, ni.PublicKey,
		hex.EncodeToString(token))
	if err != nil {
		return nil, err
	}

	// Only set the token if email verification is disabled.
	var nir v1.NewIdentityReply
	if c.cfg.SMTP == nil {
		nir.VerificationToken = hex.EncodeToString(token)
	}
	return &nir, nil
}

func (c *cmswww) HandleVerifyNewIdentity(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	vni := req.(*v1.VerifyNewIdentity)

	// Decode the verification token.
	token, err := hex.DecodeString(vni.VerificationToken)
	if err != nil {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the verification token matches.
	if !bytes.Equal(token, user.UpdateIdentityVerificationToken) {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the token hasn't expired.
	if user.UpdateIdentityVerificationExpiry < time.Now().Unix() {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenExpired,
		}
	}

	// Check signature
	sig, err := util.ConvertSignature(vni.Signature)
	if err != nil {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}

	id := user.Identities[len(user.Identities)-1]
	pi, err := identity.PublicIdentityFromBytes(id.Key[:])
	if err != nil {
		return nil, err
	}

	if !pi.VerifyMessage([]byte(vni.VerificationToken), sig) {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}

	// Associate the user id with the new public key.
	c.SetUserPubkeyAssociaton(user, pi.String())

	// Clear out the verification token fields in the db and activate
	// the key and deactivate the one it's replacing.
	user.UpdateIdentityVerificationToken = nil
	user.UpdateIdentityVerificationExpiry = 0

	t := time.Now().Unix()
	for k, v := range user.Identities {
		if v.Deactivated == 0 {
			user.Identities[k].Deactivated = t
			break
		}
	}
	user.Identities[len(user.Identities)-1].Activated = t
	user.Identities[len(user.Identities)-1].Deactivated = 0
	err = c.db.UserUpdate(user)

	return &v1.VerifyNewIdentityReply{}, err
}

/*
// ProcessUserInvoices returns the invoices for the given user.
func (c *cmswww) ProcessUserInvoices(up *v1.UserInvoices, isCurrentUser, isAdminUser bool) (*v1.UserInvoicesReply, error) {
	return &v1.UserInvoicesReply{
		Invoices: b.getInvoices(invoicesRequest{
			After:  up.After,
			Before: up.Before,
			UserID: up.UserID,
			StatusMap: map[v1.InvoiceStatusT]bool{
				v1.InvoiceStatusNotReviewed: isCurrentUser || isAdminUser,
				v1.InvoiceStatusRejected:    isCurrentUser || isAdminUser,
				v1.InvoiceStatusPublic:      true,
			},
		}),
	}, nil
}

// handleUserInvoices returns the invoices for the given user.
func (c *cmswww) handleUserInvoices(w http.ResponseWriter, r *http.Request) {
	log.Tracef("handleUserInvoices")

	// Get the user invoices command.
	var up v1.UserInvoices
	err := util.ParseGetParams(r, &up)
	if err != nil {
		RespondWithError(w, r, 0, "handleUserInvoices: ParseGetParams",
			v1.UserError{
				ErrorCode: v1.ErrorStatusInvalidInput,
			})
		return
	}

	userID, err := strconv.ParseUint(up.UserID, 10, 64)
	if err != nil {
		RespondWithError(w, r, 0, "handleUserInvoices: ParseUint",
			v1.UserError{
				ErrorCode: v1.ErrorStatusInvalidInput,
			})
		return
	}

	user, err := p.GetSessionUser(r)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleUserInvoices: GetSessionUser %v", err)
		return
	}

	upr, err := p.backend.ProcessUserInvoices(
		&up,
		user != nil && user.ID == userID,
		user != nil && user.Admin)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleUserInvoices: ProcessUserInvoices %v", err)
		return
	}

	util.RespondWithJSON(w, http.StatusOK, upr)
}
*/
