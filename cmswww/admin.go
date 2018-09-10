package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
	"github.com/decred/contractor-mgmt/cmswww/database/cockroachdb"
)

func convertWWWUserFromDatabaseUser(user database.User) v1.User {
	return v1.User{
		ID:       strconv.FormatUint(user.ID(), 10),
		Email:    user.Email(),
		Username: user.Username(),
		Admin:    user.Admin(),
		RegisterVerificationToken:        user.RegisterVerificationToken(),
		RegisterVerificationExpiry:       user.RegisterVerificationExpiry(),
		UpdateIdentityVerificationToken:  user.UpdateIdentityVerificationToken(),
		UpdateIdentityVerificationExpiry: user.UpdateIdentityVerificationExpiry(),
		LastLogin:                        user.LastLogin(),
		FailedLoginAttempts:              user.FailedLoginAttempts(),
		Locked:                           IsUserLocked(user.FailedLoginAttempts()),
		//Identities:                       convertWWWIdentitiesFromDatabaseIdentities(user.Identities),
	}
}

func convertWWWIdentitiesFromDatabaseIdentities(identities []database.Identity) []v1.UserIdentity {
	userIdentities := make([]v1.UserIdentity, 0, len(identities))
	for _, v := range identities {
		userIdentities = append(userIdentities, convertWWWIdentityFromDatabaseIdentity(v))
	}
	return userIdentities
}

func convertWWWIdentityFromDatabaseIdentity(identity database.Identity) v1.UserIdentity {
	return v1.UserIdentity{
		PublicKey: identity.EncodedKey(),
		Active:    identity.IsActive(),
	}
}

func (c *cmswww) getUserByIDStr(userIDStr string) (database.User, error) {
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return nil, err
	}

	user, err := c.db.GetUserById(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusUserNotFound,
		}
	}

	return user, nil
}

// logAdminAction logs a string to the admin log file.
//
// This function must be called WITH the mutex held.
func (c *cmswww) logAdminAction(adminUser database.User, content string) error {
	f, err := os.OpenFile(c.cfg.AdminLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	defer f.Close()

	dateTimeStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = fmt.Fprintf(f, "%v,%v,%v,%v\n", dateTimeStr,
		adminUser.ID(), adminUser.Username(), content)
	return err
}

// logAdminUserAction logs an admin action on a specific user.
//
// This function must be called WITH the mutex held.
func (c *cmswww) logAdminUserAction(adminUser, user database.User, action string, reasonForAction string) error {
	userStr := user.Username()
	if userStr == "" {
		userStr = user.Email()
	}

	return c.logAdminAction(adminUser, fmt.Sprintf("%v,%v,%v,%v",
		action, user.ID(), userStr, reasonForAction))
}

// logAdminUserAction logs an admin action on a specific user.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) logAdminUserActionLock(adminUser, user database.User, action string, reasonForAction string) error {
	c.Lock()
	defer c.Unlock()

	return c.logAdminUserAction(adminUser, user, action, reasonForAction)
}

// logAdminInvoiceAction logs an admin action on an invoice.
//
// This function must be called WITH the mutex held.
func (c *cmswww) logAdminInvoiceAction(adminUser database.User, token, action string) error {
	return c.logAdminAction(adminUser, fmt.Sprintf("%v,%v", action, token))
}

// HandleInviteNewUser creates a new user in the db if it doesn't already
// exist and sets a verification token and expiry; the token must be
// verified before it expires.
func (c *cmswww) HandleInviteNewUser(
	req interface{},
	adminUser database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	inu := req.(*v1.InviteNewUser)
	var (
		inur   v1.InviteNewUserReply
		token  []byte
		expiry int64
	)

	existingUser, err := c.db.GetUserByEmail(inu.Email)
	if err == nil {
		// Check if the user is already verified.
		if existingUser.RegisterVerificationToken() == nil {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusUserAlreadyExists,
			}
		}

		// Check if the verification token hasn't expired yet.
		if existingUser.RegisterVerificationExpiry() > time.Now().Unix() {
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
	newUser := &cockroachdb.User{}
	newUser.SetEmail(strings.ToLower(inu.Email))
	newUser.SetRegisterVerificationTokenAndExpiry(token, expiry)

	// Try to email the verification link first; if it fails, then
	// the new user won't be created.
	err = c.emailRegisterVerificationLink(inu.Email, hex.EncodeToString(token))
	if err != nil {
		return nil, err
	}

	// Save the new user in the db.
	err = c.db.NewUser(newUser)
	if err != nil {
		return nil, err
	}

	err = c.logAdminUserActionLock(adminUser, newUser, "new user invite", "")
	if err != nil {
		return nil, err
	}

	// Only set the token if email verification is disabled.
	if c.cfg.SMTP == nil {
		inur.VerificationToken = hex.EncodeToString(token)
	}
	return &inur, nil
}

func (c *cmswww) HandleUserDetails(
	req interface{},
	user database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ud := req.(*v1.UserDetails)

	// Fetch the database user.
	targetUser, err := c.findUser(ud.UserID, ud.Email, ud.Username, user.Admin())
	if err != nil {
		return nil, err
	}

	// Convert the database user into a proper response.
	var udr v1.UserDetailsReply
	udr.User = convertWWWUserFromDatabaseUser(targetUser)
	/*
		// Fetch the first page of the user's invoices.
		up := v1.UserInvoices{
			UserID: ud.UserID,
		}
		upr, err := c.ProcessUserInvoices(&up, false, true)
		if err != nil {
			return nil, err
		}

		udr.User.Invoices = upr.Invoices
	*/
	return &udr, nil
}

func (c *cmswww) HandleEditUser(
	req interface{},
	adminUser database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	eu := req.(*v1.EditUser)
	var eur v1.EditUserReply

	// Fetch the database user.
	targetUser, err := c.findUser(eu.UserID, eu.Email, eu.Username,
		adminUser.Admin())
	if err != nil {
		return nil, err
	}

	// Validate that the action is valid.
	if eu.Action == v1.UserEditInvalid {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidUserEditAction,
		}
	}

	// Validate that the reason is supplied for certain actions.
	if eu.Action == v1.UserEditLock {
		eu.Reason = strings.TrimSpace(eu.Reason)
		if len(eu.Reason) == 0 {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusReasonNotProvided,
			}
		}
	}

	switch eu.Action {
	case v1.UserEditResendInvite:
		token, err := c.resendInvite(adminUser, targetUser)
		if err != nil {
			return nil, err
		}

		eur.VerificationToken = &token
	case v1.UserEditRegenerateUpdateIdentityVerification:
		/*
			// -168 hours is 7 days in the past
			expiredTime := time.Now().Add(-168 * time.Hour).Unix()

			targetUser.UpdateIdentityVerificationExpiry = expiredTime
		*/
	case v1.UserEditUnlock:
		targetUser.SetFailedLoginAttempts(0)
		err = c.db.UpdateUser(targetUser)
		if err != nil {
			return nil, err
		}
	case v1.UserEditLock:
		targetUser.SetFailedLoginAttempts(v1.LoginAttemptsToLockUser)
		err = c.db.UpdateUser(targetUser)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported user edit action: %v",
			v1.UserEditAction[eu.Action])
	}

	// Append this action to the admin log file.
	err = c.logAdminUserActionLock(adminUser, targetUser,
		v1.UserEditAction[eu.Action], eu.Reason)
	if err != nil {
		return nil, err
	}

	return &eur, nil
}

// resendInvite sets a new verification token and expiry for a new user;
// the token must be verified before it expires.
func (c *cmswww) resendInvite(adminUser, targetUser database.User) (string, error) {
	// Check if the user is already verified.
	if targetUser.RegisterVerificationToken() == nil {
		return "", v1.UserError{
			ErrorCode: v1.ErrorStatusUserAlreadyExists,
		}
	}

	// Generate the verification token and expiry.
	token, expiry, err := c.generateVerificationTokenAndExpiry()
	if err != nil {
		return "", err
	}
	encodedToken := hex.EncodeToString(token)

	// Try to email the verification link.
	err = c.emailRegisterVerificationLink(targetUser.Email(), encodedToken)
	if err != nil {
		return "", err
	}

	targetUser.SetRegisterVerificationTokenAndExpiry(token, expiry)

	// Save the new user in the db.
	err = c.db.UpdateUser(targetUser)
	if err != nil {
		return "", err
	}

	// Only set the token if email verification is disabled.
	if c.cfg.SMTP == nil {
		return encodedToken, nil
	}
	return "", nil
}
