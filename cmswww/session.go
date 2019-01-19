package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/decred/politeia/util"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	// minimumLoginWaitTime is the minimum amount of time to wait before the
	// server sends a response to the client for the login route. This is done
	// to prevent an attacker from executing a timing attack to determine whether
	// the ErrorStatusInvalidEmailOrPassword response is specific to a bad email
	// or bad password.
	minimumLoginWaitTime = 500 * time.Millisecond
)

type loginReplyWithError struct {
	reply *v1.LoginReply
	err   error
}

// GetSession returns the active cookie session.
func (c *cmswww) getSession(r *http.Request) (*sessions.Session, error) {
	return c.store.Get(r, v1.CookieSession)
}

// GetSessionEmail returns the email address of the currently logged in user
// from the session store.
func (c *cmswww) GetSessionEmail(r *http.Request) (string, error) {
	session, err := c.getSession(r)
	if err != nil {
		return "", err
	}

	email, ok := session.Values["email"].(string)
	if !ok {
		// No email in session so return "" to indicate that.
		return "", nil
	}

	return email, nil
}

// GetSessionUser retrieves the current session user from the database.
func (c *cmswww) GetSessionUser(r *http.Request) (*database.User, error) {
	log.Tracef("GetSessionUser")
	email, err := c.GetSessionEmail(r)
	if err != nil {
		return nil, err
	}

	if len(email) == 0 {
		return nil, nil
	}
	return c.db.GetUserByEmail(email)
}

// setSessionUser sets the "email" session key to the provided value.
func (c *cmswww) setSessionUser(w http.ResponseWriter, r *http.Request, email string) error {
	log.Tracef("setSessionUser: %v %v", email, v1.CookieSession)
	session, err := c.getSession(r)
	if err != nil {
		return err
	}

	session.Values["email"] = email
	return session.Save(r, w)
}

// removeSession deletes the session from the filesystem.
func (c *cmswww) removeSession(w http.ResponseWriter, r *http.Request) error {
	log.Tracef("removeSession: %v", v1.CookieSession)
	session, err := c.getSession(r)
	if err != nil {
		return err
	}

	// Check for invalid session.
	if session.ID == "" {
		return nil
	}

	// Saving the session with a negative MaxAge will cause it to be deleted
	// from the filesystem.
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

func (c *cmswww) SetupSessions() error {
	var (
		cookieKey []byte
		err       error
	)

	// Persist session cookies.
	if cookieKey, err = ioutil.ReadFile(c.cfg.CookieKeyFile); err != nil {
		log.Infof("Cookie key not found, generating one...")
		cookieKey, err = util.Random(32)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(c.cfg.CookieKeyFile, cookieKey, 0400)
		if err != nil {
			return err
		}
		log.Infof("Cookie key generated: %v", cookieKey)
	}
	sessionsDir := filepath.Join(c.cfg.DataDir, "sessions")
	err = os.MkdirAll(sessionsDir, 0700)
	if err != nil {
		return err
	}
	c.store = sessions.NewFilesystemStore(sessionsDir, cookieKey)
	c.store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400, // One day
		Secure:   true,
		HttpOnly: true,
	}

	return nil
}

func (c *cmswww) ReplaceSessionIfInvalid(w http.ResponseWriter, r *http.Request) (bool, error) {
	session, err := c.getSession(r)
	if err != nil && session != nil {
		// Create and save a new session for the user.
		session := sessions.NewSession(c.store, v1.CookieSession)
		opts := *c.store.Options
		session.Options = &opts
		session.IsNew = true
		return true, session.Save(r, w)
	}

	return false, nil
}

// isAdmin returns true if the current session has admin privileges.
func (c *cmswww) isAdmin(r *http.Request) (bool, error) {
	user, err := c.GetSessionUser(r)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	return user.Admin, nil
}

func (c *cmswww) login(l *v1.Login) loginReplyWithError {
	// Get user from db.
	user, err := c.db.GetUserByEmail(l.Email)
	if err != nil {
		if err == database.ErrUserNotFound {
			return loginReplyWithError{
				reply: nil,
				err: v1.UserError{
					ErrorCode: v1.ErrorStatusInvalidEmailOrPassword,
				},
			}
		}
		return loginReplyWithError{
			reply: nil,
			err:   err,
		}
	}

	// Check that the user is verified.
	if user.IsVerified() {
		return loginReplyWithError{
			reply: nil,
			err: v1.UserError{
				ErrorCode: v1.ErrorStatusInvalidEmailOrPassword,
			},
		}
	}

	// Check the user's password.
	err = bcrypt.CompareHashAndPassword(user.HashedPassword,
		[]byte(l.Password))
	if err != nil {
		if !IsUserLocked(user.FailedLoginAttempts) {
			user.FailedLoginAttempts = user.FailedLoginAttempts + 1
			err := c.db.UpdateUser(user)
			if err != nil {
				return loginReplyWithError{
					reply: nil,
					err:   err,
				}
			}

			// Check if the user is locked again so we can send an email.
			if IsUserLocked(user.FailedLoginAttempts) {
				// This is conditional on the email server being setup.
				err := c.emailUserLocked(user.Email)
				if err != nil {
					return loginReplyWithError{
						reply: nil,
						err:   err,
					}
				}
			}
		}

		return loginReplyWithError{
			reply: nil,
			err: v1.UserError{
				ErrorCode: v1.ErrorStatusInvalidEmailOrPassword,
			},
		}
	}

	// Check if user is locked due to too many login attempts
	if IsUserLocked(user.FailedLoginAttempts) {
		return loginReplyWithError{
			reply: nil,
			err: v1.UserError{
				ErrorCode: v1.ErrorStatusUserLocked,
			},
		}
	}

	lastLogin := user.LastLogin
	user.FailedLoginAttempts = 0
	user.LastLogin = time.Now().Unix()
	err = c.db.UpdateUser(user)
	if err != nil {
		return loginReplyWithError{
			reply: nil,
			err:   err,
		}
	}

	reply, err := c.CreateLoginReply(user, lastLogin)
	return loginReplyWithError{
		reply: reply,
		err:   err,
	}
}

// ProcessLogin checks that a user exists, is verified, and has
// the correct password.
func (c *cmswww) HandleLogin(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	l := req.(*v1.Login)
	var (
		loginReply loginReplyWithError
		login      = make(chan loginReplyWithError)
		timeout    = make(chan bool)
	)

	go func() {
		login <- c.login(l)
	}()
	go func() {
		time.Sleep(minimumLoginWaitTime)
		timeout <- true
	}()

	// Execute both goroutines in parallel, and only return
	// when both are finished.
	select {
	case loginReply = <-login:
	case <-timeout:
	}

	select {
	case loginReply = <-login:
	case <-timeout:
	}

	if loginReply.err == nil {
		// Mark user as logged in if there's no error.
		err := c.setSessionUser(w, r, l.Email)
		if err != nil {
			return nil, err
		}
	}

	return loginReply.reply, loginReply.err
}

func (c *cmswww) CreateLoginReply(user *database.User, lastLogin int64) (*v1.LoginReply, error) {
	activeIdentity, ok := database.ActiveIdentityString(user.Identities)
	if !ok {
		activeIdentity = ""
	}

	reply := v1.LoginReply{
		IsAdmin:   user.Admin,
		UserID:    user.ID.String(),
		Email:     user.Email,
		Username:  user.Username,
		PublicKey: activeIdentity,
		LastLogin: lastLogin,
	}

	return &reply, nil
}

// HandleLogout logs the user out.
func (c *cmswww) HandleLogout(w http.ResponseWriter, r *http.Request) {
	log.Tracef("HandleLogout")

	err := c.removeSession(w, r)
	if err != nil {
		RespondWithError(w, r, 0,
			"HandleLogout: removeSession %v", err)
		return
	}

	// Reply with the user information.
	var reply v1.LogoutReply
	util.RespondWithJSON(w, http.StatusOK, reply)
}
