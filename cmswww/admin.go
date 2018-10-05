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
)

func (c *cmswww) getUserByIDStr(userIDStr string) (*database.User, error) {
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
func (c *cmswww) logAdminAction(adminUser *database.User, content string) error {
	f, err := os.OpenFile(c.cfg.AdminLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	defer f.Close()

	dateTimeStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = fmt.Fprintf(f, "%v,%v,%v,%v\n", dateTimeStr,
		adminUser.ID, adminUser.Username, content)
	return err
}

// logAdminUserAction logs an admin action on a specific user.
//
// This function must be called WITH the mutex held.
func (c *cmswww) logAdminUserAction(adminUser, user *database.User, action string, reasonForAction string) error {
	userStr := user.Username
	if userStr == "" {
		userStr = user.Email
	}

	return c.logAdminAction(adminUser, fmt.Sprintf("%v,%v,%v,%v",
		action, user.ID, userStr, reasonForAction))
}

// logAdminUserAction logs an admin action on a specific user.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) logAdminUserActionLock(adminUser, user *database.User, action string, reasonForAction string) error {
	c.Lock()
	defer c.Unlock()

	return c.logAdminUserAction(adminUser, user, action, reasonForAction)
}

// logAdminInvoiceAction logs an admin action on an invoice.
//
// This function must be called WITH the mutex held.
func (c *cmswww) logAdminInvoiceAction(adminUser *database.User, token, action string) error {
	return c.logAdminAction(adminUser, fmt.Sprintf("%v,%v", action, token))
}

// HandleInviteNewUser creates a new user in the db if it doesn't already
// exist and sets a verification token and expiry; the token must be
// verified before it expires.
func (c *cmswww) HandleInviteNewUser(
	req interface{},
	adminUser *database.User,
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
		if !existingUser.IsVerified() {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusUserAlreadyExists,
			}
		}

		// Check if the verification token hasn't expired yet.
		if existingUser.RegisterVerificationExpiry > time.Now().Unix() {
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
	newUser := &database.User{}
	newUser.Email = strings.ToLower(inu.Email)
	newUser.RegisterVerificationToken = token
	newUser.RegisterVerificationExpiry = expiry

	// Try to email the verification link first; if it fails, then
	// the new user won't be created.
	err = c.emailRegisterVerificationLink(inu.Email, hex.EncodeToString(token))
	if err != nil {
		return nil, err
	}

	// Save the new user in the db.
	err = c.db.CreateUser(newUser)
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
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ud := req.(*v1.UserDetails)

	// Fetch the database user.
	targetUser, err := c.findUser(ud.UserID, ud.Email, ud.Username, user.Admin)
	if err != nil {
		return nil, err
	}

	var udr v1.UserDetailsReply
	if targetUser == nil {
		return &udr, nil
	}
	if !user.Admin && targetUser.ID != user.ID {
		// Don't return user details for another user unless the requesting
		// user is an admin.
		return &udr, nil
	}

	// Convert the database user into a proper response.
	udr.User = convertDatabaseUserToUser(targetUser)
	return &udr, nil
}

func (c *cmswww) HandleManageUser(
	req interface{},
	adminUser *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	mu := req.(*v1.ManageUser)
	var mur v1.ManageUserReply

	// Fetch the database user.
	targetUser, err := c.findUser(mu.UserID, mu.Email, mu.Username,
		adminUser.Admin)
	if err != nil {
		return nil, err
	}

	// Validate that the action is valid.
	if mu.Action == v1.UserManageInvalid {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidUserManageAction,
		}
	}

	// Validate that the reason is supplied for certain actions.
	if mu.Action == v1.UserManageLock {
		mu.Reason = strings.TrimSpace(mu.Reason)
		if len(mu.Reason) == 0 {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusReasonNotProvided,
			}
		}
	}

	switch mu.Action {
	case v1.UserManageResendInvite:
		token, err := c.resendInvite(adminUser, targetUser)
		if err != nil {
			return nil, err
		}

		mur.VerificationToken = &token
	case v1.UserManageRegenerateUpdateIdentityVerification:
		/*
			// -168 hours is 7 days in the past
			expiredTime := time.Now().Add(-168 * time.Hour).Unix()

			targetUser.UpdateIdentityVerificationExpiry = expiredTime
		*/
	case v1.UserManageUnlock:
		targetUser.FailedLoginAttempts = 0
		err = c.db.UpdateUser(targetUser)
		if err != nil {
			return nil, err
		}
	case v1.UserManageLock:
		targetUser.FailedLoginAttempts = v1.LoginAttemptsToLockUser
		err = c.db.UpdateUser(targetUser)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported user manage action: %v",
			v1.UserManageAction[mu.Action])
	}

	// Append this action to the admin log file.
	err = c.logAdminUserActionLock(adminUser, targetUser,
		v1.UserManageAction[mu.Action], mu.Reason)
	if err != nil {
		return nil, err
	}

	return &mur, nil
}

// resendInvite sets a new verification token and expiry for a new user;
// the token must be verified before it expires.
func (c *cmswww) resendInvite(adminUser, targetUser *database.User) (string, error) {
	// Check if the user is already verified.
	if !targetUser.IsVerified() {
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
	err = c.emailRegisterVerificationLink(targetUser.Email, encodedToken)
	if err != nil {
		return "", err
	}

	targetUser.RegisterVerificationToken = token
	targetUser.RegisterVerificationExpiry = expiry

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
