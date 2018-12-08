// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package database

import (
	"encoding/hex"
	"errors"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/politeia/politeiad/api/v1/identity"
)

var (
	// ErrUserNotFound indicates that a user name was not found in the
	// database.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvoiceNotFound indicates that the invoice was not found in the
	// database.
	ErrInvoiceNotFound = errors.New("invoice not found")

	// ErrUserExists indicates that a user already exists in the database.
	ErrUserExists = errors.New("user already exists")

	// ErrInvalidEmail indicates that a user's email is not properly formatted.
	ErrInvalidEmail = errors.New("invalid user email")

	// ErrShutdown is emitted when the database is shutting down.
	ErrShutdown = errors.New("database is shutting down")
)

// InvoicesRequest is used for passing parameters into the
// GetInvoices() function.
type InvoicesRequest struct {
	UserID    string
	Month     uint16
	Year      uint16
	StatusMap map[v1.InvoiceStatusT]bool
	Page      int
}

// Database interface that is required by the web server.
type Database interface {
	// User functions
	CreateUser(*User) error                                  // Create new user
	UpdateUser(*User) error                                  // Update existing user
	GetUserByEmail(string) (*User, error)                    // Return user record given the email address
	GetUserByUsername(string) (*User, error)                 // Return user record given the username
	GetUserById(uint64) (*User, error)                       // Return user record given its id
	GetUserIdByPublicKey(string) (uint64, error)             // Return user id by public key
	GetAllUsers(callbackFn func(u *User)) error              // Iterate all users
	GetUsers(username string, page int) ([]User, int, error) // Returns a list of users and total count that match the provided username.

	// Invoice functions
	CreateInvoice(*Invoice) error                        // Create new invoice
	UpdateInvoice(*Invoice) error                        // Update existing invoice
	GetInvoiceByToken(string) (*Invoice, error)          // Return invoice given its token
	GetInvoices(InvoicesRequest) ([]Invoice, int, error) // Return a list of invoices
	UpdateInvoicePayment(*InvoicePayment) error          // Update an existing invoice's payment

	DeleteAllData() error // Delete all data from all tables

	// Close performs cleanup of the backend.
	Close() error
}

// User record.
type User struct {
	ID                                        uint64
	Email                                     string
	Username                                  string
	Name                                      string
	Location                                  string
	ExtendedPublicKey                         string
	HashedPassword                            []byte
	Admin                                     bool
	RegisterVerificationToken                 []byte
	RegisterVerificationExpiry                int64
	UpdateIdentityVerificationToken           []byte
	UpdateIdentityVerificationExpiry          int64
	ResetPasswordVerificationToken            []byte
	ResetPasswordVerificationExpiry           int64
	UpdateExtendedPublicKeyVerificationToken  []byte
	UpdateExtendedPublicKeyVerificationExpiry int64
	LastLogin                                 int64
	FailedLoginAttempts                       uint64
	PaymentAddressIndex                       uint64
	EmailNotifications                        uint64

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

type Invoice struct {
	Token              string
	UserID             uint64
	Username           string // Only populated when reading from the database
	Month              uint16
	Year               uint16
	Timestamp          int64
	Status             v1.InvoiceStatusT
	StatusChangeReason string
	File               *File
	PublicKey          string
	UserSignature      string
	ServerSignature    string
	Proposal           string // Optional link to a Politeia proposal
	Version            string // Version number of this invoice

	Changes  []InvoiceChange
	Payments []InvoicePayment
}

type File struct {
	Payload string
	MIME    string
	Digest  string
}

type InvoiceChange struct {
	AdminPublicKey string
	NewStatus      v1.InvoiceStatusT
	Reason         string
	Timestamp      int64
}

type InvoicePayment struct {
	ID           uint64
	InvoiceToken string
	Address      string
	Amount       uint64
	TxNotBefore  int64
	PollExpiry   int64
	TxID         string
}

func (id *Identity) IsActive() bool {
	return id.Activated != 0 && id.Deactivated == 0
}

func (u *User) IsVerified() bool {
	return u.RegisterVerificationToken != nil && len(u.RegisterVerificationToken) > 0
}

// ActiveIdentity returns a the current active key.  If there is no active
// valid key the call returns all 0s and false.
func ActiveIdentity(ids []Identity) ([identity.PublicKeySize]byte, bool) {
	for _, id := range ids {
		if id.IsActive() {
			return id.Key, true
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
