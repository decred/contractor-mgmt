package main

import (
	"bytes"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/decred/politeia/politeiad/api/v1/identity"
	"github.com/decred/politeia/util"
	"github.com/gofrs/uuid"
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
	id, err := uuid.FromString(idStr)
	if err != nil {
		return ""
	}

	user, err := c.db.GetUserById(id)
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
		id, err := uuid.FromString(idStr)
		if err == nil {
			user, err = c.db.GetUserById(id)
			if err != nil && err != database.ErrUserNotFound {
				return nil, err
			}
		}
	}

	if user == nil && isAdmin && email != "" {
		user, err = c.db.GetUserByEmail(email)
		if err != nil && err != database.ErrUserNotFound {
			return nil, err
		}
	}

	if user == nil && username != "" {
		user, err = c.db.GetUserByUsername(username)
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
	user, err := c.db.GetUserByEmail(nu.Email)
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
	if user.RegisterVerificationExpiry < time.Now().Unix() {
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

	// Update the user in the db.
	user.RegisterVerificationToken = []byte{}
	user.RegisterVerificationExpiry = 0
	user.HashedPassword = hashedPassword
	user.Username = nu.Username
	user.Name = nu.Name
	user.Location = nu.Location
	user.ExtendedPublicKey = nu.ExtendedPublicKey

	id := database.Identity{}
	id.Activated = time.Now().Unix()
	id.Key = [identity.PublicKeySize]byte{}
	copy(id.Key[:], pk)
	user.Identities = append(user.Identities, id)

	err = c.db.UpdateUser(user)
	return &v1.RegisterReply{}, err
}

// HandleEditUser changes a user's details.
func (c *cmswww) HandleEditUser(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	eu := req.(*v1.EditUser)

	// Update the user in the db.
	if eu.Name != nil {
		user.Name = *eu.Name
	}
	if eu.Location != nil {
		user.Location = *eu.Location
	}
	if eu.EmailNotifications != nil {
		user.EmailNotifications = *eu.EmailNotifications
	}

	err := c.db.UpdateUser(user)
	return &v1.EditUserReply{}, err
}

// HandleEditUserExtendedPublicKey changes a user's extended public key.
func (c *cmswww) HandleEditUserExtendedPublicKey(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	eu := req.(*v1.EditUserExtendedPublicKey)
	var eur v1.EditUserExtendedPublicKeyReply

	if eu.VerificationToken == "" {
		c.emailUpdateExtendedPublicKey(user, &eur)
	} else {
		token := hex.EncodeToString(user.UpdateExtendedPublicKeyVerificationToken)
		if eu.VerificationToken != token {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
			}
		}

		if user.UpdateExtendedPublicKeyVerificationExpiry < time.Now().Unix() {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusVerificationTokenExpired,
			}
		}

		user.ExtendedPublicKey = eu.ExtendedPublicKey
		user.UpdateExtendedPublicKeyVerificationToken = []byte{}
		user.UpdateExtendedPublicKeyVerificationExpiry = 0
		err := c.db.UpdateUser(user)
		if err != nil {
			return nil, err
		}
	}

	return &eur, nil
}

func (c *cmswww) emailUpdateExtendedPublicKey(
	user *database.User,
	eur *v1.EditUserExtendedPublicKeyReply,
) error {
	if user.UpdateExtendedPublicKeyVerificationToken != nil {
		if user.UpdateExtendedPublicKeyVerificationExpiry > time.Now().Unix() {
			// The verification token is present and hasn't expired, so do nothing.
			return nil
		}
	}

	// The verification token isn't present or is present but expired.

	// Generate a new verification token and expiry.
	token, expiry, err := c.generateVerificationTokenAndExpiry()
	if err != nil {
		return err
	}

	// Add the updated user information to the db.
	user.UpdateExtendedPublicKeyVerificationToken = token
	user.UpdateExtendedPublicKeyVerificationExpiry = expiry
	err = c.db.UpdateUser(user)
	if err != nil {
		return err
	}

	// This is conditional on the email server being setup.
	err = c.emailUpdateExtendedPublicKeyVerificationLink(user.Email,
		hex.EncodeToString(token))
	if err != nil {
		return err
	}

	// Only set the token if email verification is disabled.
	if c.cfg.SMTP == nil {
		eur.VerificationToken = hex.EncodeToString(token)
	}

	return nil
}

func (c *cmswww) verifyUpdateExtendedPublicKey(
	user *database.User,
	rp *v1.ResetPassword,
	rpr *v1.ResetPasswordReply,
) error {
	// Decode the verification token.
	token, err := hex.DecodeString(rp.VerificationToken)
	if err != nil {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the verification token matches.
	if !bytes.Equal(token, user.ResetPasswordVerificationToken) {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the token hasn't expired.
	if user.ResetPasswordVerificationExpiry < time.Now().Unix() {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenExpired,
		}
	}

	// Validate the new password.
	err = validatePassword(rp.NewPassword)
	if err != nil {
		return err
	}

	// Hash the new password.
	hashedPassword, err := hashPassword(rp.NewPassword)
	if err != nil {
		return err
	}

	// Clear out the verification token fields, set the new password in the db,
	// and unlock account
	user.ResetPasswordVerificationToken = []byte{}
	user.ResetPasswordVerificationExpiry = 0
	user.HashedPassword = hashedPassword
	user.FailedLoginAttempts = 0

	return c.db.UpdateUser(user)
}

// HandleNewIdentity creates a new identity.
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
					strconv.FormatInt(user.UpdateIdentityVerificationExpiry,
						10),
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

	id := database.Identity{}
	id.Key = [identity.PublicKeySize]byte{}
	copy(id.Key[:], pk)
	user.Identities = append(user.Identities, id)

	err = c.db.UpdateUser(user)
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

// HandleVerifyNewIdentity verifies a newly generated identity.
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
	key := id.Key
	pi, err := identity.PublicIdentityFromBytes(key[:])
	if err != nil {
		return nil, err
	}

	if !pi.VerifyMessage([]byte(vni.VerificationToken), sig) {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidSignature,
		}
	}

	// Clear out the verification token fields in the db and activate
	// the key and deactivate the one it's replacing.
	user.UpdateIdentityVerificationToken = []byte{}
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
	err = c.db.UpdateUser(user)

	return &v1.VerifyNewIdentityReply{}, err
}

// HandleChangePassword checks that the current password matches the one
// in the database, then changes it to the new password.
func (c *cmswww) HandleChangePassword(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	cp := req.(*v1.ChangePassword)

	// Check the user's password.
	err := bcrypt.CompareHashAndPassword(user.HashedPassword,
		[]byte(cp.CurrentPassword))
	if err != nil {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidEmailOrPassword,
		}
	}

	// Validate the new password.
	err = validatePassword(cp.NewPassword)
	if err != nil {
		return nil, err
	}

	// Hash the user's password.
	hashedPassword, err := hashPassword(cp.NewPassword)
	if err != nil {
		return nil, err
	}

	// Add the updated user information to the db.
	user.HashedPassword = hashedPassword
	err = c.db.UpdateUser(user)
	if err != nil {
		return nil, err
	}

	return &v1.ChangePasswordReply{}, nil
}

// HandleResetPassword is intended to be called twice; in the first call, an
// email is provided and the function checks if the user exists. If the user exists, it
// generates a verification token and stores it in the database. In the second
// call, the email, verification token and a new password are provided. If everything
// matches, then the user's password is updated in the database.
func (c *cmswww) HandleResetPassword(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	rp := req.(*v1.ResetPassword)
	rpr := &v1.ResetPasswordReply{}

	// Get user from db.
	var err error
	user, err = c.db.GetUserByEmail(rp.Email)
	if err != nil {
		if err == database.ErrInvalidEmail {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusMalformedEmail,
			}
		} else if err == database.ErrUserNotFound {
			return &rpr, nil
		}

		return nil, err
	}

	if rp.VerificationToken == "" {
		err = c.emailResetPassword(user, rp, rpr)
	} else {
		err = c.verifyResetPassword(user, rp, rpr)
	}

	if err != nil {
		return nil, err
	}

	return rpr, nil
}

func (c *cmswww) emailResetPassword(
	user *database.User,
	rp *v1.ResetPassword,
	rpr *v1.ResetPasswordReply,
) error {
	if user.ResetPasswordVerificationToken != nil {
		if user.ResetPasswordVerificationExpiry > time.Now().Unix() {
			// The verification token is present and hasn't expired, so do nothing.
			return nil
		}
	}

	// The verification token isn't present or is present but expired.

	// Generate a new verification token and expiry.
	token, expiry, err := c.generateVerificationTokenAndExpiry()
	if err != nil {
		return err
	}

	// Add the updated user information to the db.
	user.ResetPasswordVerificationToken = token
	user.ResetPasswordVerificationExpiry = expiry
	err = c.db.UpdateUser(user)
	if err != nil {
		return err
	}

	// This is conditional on the email server being setup.
	err = c.emailResetPasswordVerificationLink(rp.Email, hex.EncodeToString(token))
	if err != nil {
		return err
	}

	// Only set the token if email verification is disabled.
	if c.cfg.SMTP == nil {
		rpr.VerificationToken = hex.EncodeToString(token)
	}

	return nil
}

func (c *cmswww) verifyResetPassword(
	user *database.User,
	rp *v1.ResetPassword,
	rpr *v1.ResetPasswordReply,
) error {
	// Decode the verification token.
	token, err := hex.DecodeString(rp.VerificationToken)
	if err != nil {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the verification token matches.
	if !bytes.Equal(token, user.ResetPasswordVerificationToken) {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenInvalid,
		}
	}

	// Check that the token hasn't expired.
	if user.ResetPasswordVerificationExpiry < time.Now().Unix() {
		return v1.UserError{
			ErrorCode: v1.ErrorStatusVerificationTokenExpired,
		}
	}

	// Validate the new password.
	err = validatePassword(rp.NewPassword)
	if err != nil {
		return err
	}

	// Hash the new password.
	hashedPassword, err := hashPassword(rp.NewPassword)
	if err != nil {
		return err
	}

	// Clear out the verification token fields, set the new password in the db,
	// and unlock account
	user.ResetPasswordVerificationToken = []byte{}
	user.ResetPasswordVerificationExpiry = 0
	user.HashedPassword = hashedPassword
	user.FailedLoginAttempts = 0

	return c.db.UpdateUser(user)
}
