// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package database

import (
	"encoding/hex"
	"errors"

	"github.com/decred/politeia/politeiad/api/v1/identity"
)

var (
	// ErrUserNotFound indicates that a user name was not found in the
	// database.
	ErrUserNotFound = errors.New("user not found")

	// ErrUserExists indicates that a user already exists in the database.
	ErrUserExists = errors.New("user already exists")

	// ErrInvalidEmail indicates that a user's email is not properly formatted.
	ErrInvalidEmail = errors.New("invalid user email")

	// ErrShutdown is emitted when the database is shutting down.
	ErrShutdown = errors.New("database is shutting down")
)

// Database interface that is required by the web server.
type Database interface {
	// User functions
	GetUserByEmail(string) (*User, error)    // Return user record given the email address
	GetUserByUsername(string) (*User, error) // Return user record given the username
	GetUserById(uint64) (*User, error)       // Return user record given its id
	NewUser(*User) error                     // Add new user
	UpdateUser(*User) error                  // Update existing user
	AllUsers(callbackFn func(u *User)) error // Iterate all users
	DeleteAllData() error                    // Delete all data from all tables

	// Close performs cleanup of the backend.
	Close() error
}

// User record.
type User struct {
	ID                               uint64
	Email                            string
	Username                         string
	HashedPassword                   []byte
	Admin                            bool
	RegisterVerificationToken        []byte
	RegisterVerificationExpiry       int64
	UpdateIdentityVerificationToken  []byte
	UpdateIdentityVerificationExpiry int64
	LastLogin                        int64
	FailedLoginAttempts              uint64

	Identities []Identity
}

// Identity wraps an ed25519 public key and timestamps to indicate if it is
// active.  If deactivated != 0 then the key is no longer valid.
type Identity struct {
	ID          uint64
	UserID      uint64
	Key         [identity.PublicKeySize]byte
	Activated   int64
	Deactivated int64
}

func IsActive(id Identity) bool {
	return id.Activated != 0 && id.Deactivated == 0
}

// ActiveIdentity returns a the current active key.  If there is no active
// valid key the call returns all 0s and false.
func ActiveIdentity(i []Identity) ([identity.PublicKeySize]byte, bool) {
	for _, v := range i {
		if IsActive(v) {
			return v.Key, true
		}
	}

	return [identity.PublicKeySize]byte{}, false
}

// ActiveIdentityString returns a string representation of the current active
// key.  If there is no active valid key the call returns all 0s and false.
func ActiveIdentityString(i []Identity) (string, bool) {
	key, ok := ActiveIdentity(i)
	return hex.EncodeToString(key[:]), ok
}
