package main

import (
	"bytes"
	"encoding/hex"
	"github.com/decred/politeia/politeiad/api/v1/identity"
	"net/http"
	"strconv"
	"strings"
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

// HandleGenerateNewUser creates a new user in the db if it doesn't already
// exist and sets a verification token and expiry; the token must be
// verified before it expires.
func (c *cmswww) HandleGenerateNewUser(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	gnu := req.(*v1.GenerateNewUser)
	var (
		gnur   v1.GenerateNewUserReply
		token  []byte
		expiry int64
	)

	existingUser, err := c.db.UserGet(gnu.Email)
	if err == nil {
		// Check if the user is already verified.
		if existingUser.NewUserVerificationToken == nil {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusUserAlreadyExists,
			}
		}

		// Check if the verification token hasn't expired yet.
		if existingUser.NewUserVerificationExpiry > time.Now().Unix() {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusVerificationTokenUnexpired,
			}
		}
	}

	// Generate the verification token and expiry.
	token, expiry, err = c.generateVerificationTokenAndExpiry()
	if err != nil {
		return nil, err
	}

	// Create a new database user with the provided information.
	newUser := database.User{
		Email: strings.ToLower(gnu.Email),
		Admin: false,
		NewUserVerificationToken:  token,
		NewUserVerificationExpiry: expiry,
	}

	// Try to email the verification link first; if it fails, then
	// the new user won't be created.
	//
	// This is conditional on the email server being setup.
	err = c.emailNewUserVerificationLink(gnu.Email, hex.EncodeToString(token))
	if err != nil {
		return nil, err
	}

	// Save the new user in the db.
	err = c.db.UserNew(newUser)
	if err != nil {
		return nil, err
	}

	// Only set the token if email verification is disabled.
	if c.cfg.SMTP == nil {
		gnur.VerificationToken = hex.EncodeToString(token)
	}
	return &gnur, nil
}

// HandleNewUser verifies the token generated for a recently created
// user.  It ensures that the token matches with the input and that the token
// hasn't expired.  On success it returns database user record.
func (c *cmswww) HandleNewUser(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	nu := req.(*v1.NewUser)

	// Check that the user already exists.
	user, err := c.db.UserGet(nu.Email)
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
	if !bytes.Equal(token, user.NewUserVerificationToken) {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the token hasn't expired.
	if time.Now().Unix() > user.NewUserVerificationExpiry {
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
	err = c.validateUsername(nu.Username, user)
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
	user.NewUserVerificationToken = nil
	user.NewUserVerificationExpiry = 0
	user.HashedPassword = hashedPassword
	user.Username = nu.Username
	user.Identities = []database.Identity{{
		Activated: time.Now().Unix(),
	}}
	copy(user.Identities[0].Key[:], pk)

	err = c.db.UserUpdate(*user)
	return &v1.NewUserReply{}, err
}

/*
// ProcessUserInvoices returns the invoices for the given user.
func (c *cmswww) ProcessUserInvoices(up *v1.UserInvoices, isCurrentUser, isAdminUser bool) (*v1.UserInvoicesReply, error) {
	return &v1.UserInvoicesReply{
		Invoices: b.getInvoices(invoicesRequest{
			After:  up.After,
			Before: up.Before,
			UserId: up.UserId,
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

	userId, err := strconv.ParseUint(up.UserId, 10, 64)
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
		user != nil && user.ID == userId,
		user != nil && user.Admin)
	if err != nil {
		RespondWithError(w, r, 0,
			"handleUserInvoices: ProcessUserInvoices %v", err)
		return
	}

	util.RespondWithJSON(w, http.StatusOK, upr)
}
*/
