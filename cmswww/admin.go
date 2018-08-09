package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

func convertWWWUserFromDatabaseUser(user *database.User) v1.User {
	return v1.User{
		ID:       strconv.FormatUint(user.ID, 10),
		Email:    user.Email,
		Username: user.Username,
		Admin:    user.Admin,
		RegisterVerificationToken:        user.RegisterVerificationToken,
		RegisterVerificationExpiry:       user.RegisterVerificationExpiry,
		UpdateIdentityVerificationToken:  user.UpdateIdentityVerificationToken,
		UpdateIdentityVerificationExpiry: user.UpdateIdentityVerificationExpiry,
		LastLogin:                        user.LastLogin,
		FailedLoginAttempts:              user.FailedLoginAttempts,
		Locked:                           IsUserLocked(user.FailedLoginAttempts),
		Identities:                       convertWWWIdentitiesFromDatabaseIdentities(user.Identities),
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
		PublicKey: hex.EncodeToString(identity.Key[:]),
		Active:    database.IsIdentityActive(identity),
	}
}

func (c *cmswww) getUserByIDStr(userIDStr string) (*database.User, error) {
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return nil, err
	}

	user, err := c.db.UserGetById(userID)
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
func (c *cmswww) logAdminUserAction(adminUser, user *database.User, action v1.UserEditActionT, reasonForAction string) error {
	return c.logAdminAction(adminUser, fmt.Sprintf("%v,%v,%v,%v",
		v1.UserEditAction[action], user.ID, user.Username, reasonForAction))
}

// logAdminUserAction logs an admin action on a specific user.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) logAdminUserActionLock(adminUser, user *database.User, action v1.UserEditActionT, reasonForAction string) error {
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

func (c *cmswww) SetUserDetailsPathParams(
	req interface{},
	w http.ResponseWriter,
	r *http.Request,
) error {
	ud := req.(*v1.UserDetails)

	pathParams := mux.Vars(r)
	ud.UserID = pathParams["userid"]
	return nil
}

func (c *cmswww) HandleUserDetails(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	ud := req.(*v1.UserDetails)

	// Fetch the database user.
	user, err := c.getUserByIDStr(ud.UserID)
	if err != nil {
		return nil, err
	}

	// Convert the database user into a proper response.
	var udr v1.UserDetailsReply
	udr.User = convertWWWUserFromDatabaseUser(user)
	/*
		// Fetch the first page of the user's invoices.
		up := v1.UserInvoices{
			UserId: ud.UserID,
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
	adminUser *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	eu := req.(*v1.EditUser)

	// Fetch the database user.
	targetUser, err := c.getUserByIDStr(eu.UserID)
	if err != nil {
		return nil, err
	}

	// Validate that the action is valid.
	if eu.Action == v1.UserEditInvalid {
		return nil, v1.UserError{
			ErrorCode: v1.ErrorStatusInvalidUserEditAction,
		}
	}

	// Validate that the reason is supplied.
	if eu.Action == v1.UserEditLock {
		eu.Reason = strings.TrimSpace(eu.Reason)
		if len(eu.Reason) == 0 {
			return nil, v1.UserError{
				ErrorCode: v1.ErrorStatusInvalidInput,
			}
		}
	}

	// Append this action to the admin log file.
	err = c.logAdminUserActionLock(adminUser, targetUser, eu.Action, eu.Reason)
	if err != nil {
		return nil, err
	}

	// -168 hours is 7 days in the past
	expiredTime := time.Now().Add(-168 * time.Hour).Unix()

	switch eu.Action {
	case v1.UserEditRegenerateRegisterVerification:
		targetUser.RegisterVerificationExpiry = expiredTime
	case v1.UserEditRegenerateUpdateIdentityVerification:
		targetUser.UpdateIdentityVerificationExpiry = expiredTime
	case v1.UserEditUnlock:
		targetUser.FailedLoginAttempts = 0
	case v1.UserEditLock:
		targetUser.FailedLoginAttempts = v1.LoginAttemptsToLockUser
	default:
		return nil, fmt.Errorf("unsupported user edit action: %v",
			v1.UserEditAction[eu.Action])
	}

	// Update the user in the database.
	err = c.db.UserUpdate(*targetUser)
	return &v1.EditUserReply{}, err
}
