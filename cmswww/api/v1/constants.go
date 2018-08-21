package v1

import (
	"time"
)

const (
	APIVersion = 1 // API version this backend understands

	// CsrfToken is the CSRF header
	CsrfToken = "X-CSRF-Token"

	// Forward is the proxy header
	Forward = "X-Forwarded-For"

	// CookieSession is the cookie name that indicates that a user is
	// logged in.
	CookieSession = "session"

	// VerificationTokenSize is the size of verification token in bytes
	VerificationTokenSize = 32

	// VerificationExpiryTime is the amount of time before the expiration token
	// expires
	VerificationExpiryTime = 48 * time.Hour

	// PolicyMinPasswordLength is the minimum number of characters
	// accepted for user passwords
	PolicyMinPasswordLength = 8

	// PolicyMaxUsernameLength is the max length of a username
	PolicyMaxUsernameLength = 30

	// PolicyMinUsernameLength is the min length of a username
	PolicyMinUsernameLength = 3

	// PolicyInvoiceCommentChar is the character which, when used as the first
	// character of a line, denotes that entire line as a comment.
	PolicyInvoiceCommentChar rune = '#'

	// PolicyInvoiceFieldDelimiterChar is the character that delimits field
	// values for each line item in the CSV.
	PolicyInvoiceFieldDelimiterChar rune = ','

	// ListPageSize is the maximum number of entries returned
	// for the routes that return lists
	ListPageSize = 25

	// LoginAttemptsToLockUser is the number of consecutive failed
	// login attempts permitted before the system locks the user.
	LoginAttemptsToLockUser = 5
)

type InvoiceFieldDetails struct {
	Name     string            `json:"name"`
	Type     InvoiceFieldTypeT `json:"type"`
	Required bool              `json:"required"`
}

var (
	// PolicyInvoiceNameSupportedChars is the regular expression of a valid
	// invoice name
	PolicyInvoiceNameSupportedChars = []string{
		"A-z", "0-9", "&", ".", ",", ":", ";", "-", " ", "@", "+", "#", "/",
		"(", ")", "!", "?", "\"", "'"}

	// PolicyUsernameSupportedChars is the regular expression of a valid
	// username
	PolicyUsernameSupportedChars = []string{
		"A-z", "0-9", ".", ",", ":", ";", "-", " ", "@", "+",
		"(", ")"}

	// InvoiceFields is the list of fields for each line item in an invoice.
	InvoiceFields = []InvoiceFieldDetails{
		{
			Name:     "Description of work",
			Type:     InvoiceFieldTypeString,
			Required: true,
		},
		{
			Name:     "Type of work",
			Type:     InvoiceFieldTypeString,
			Required: true,
		},
		{
			Name:     "Subtype of work",
			Type:     InvoiceFieldTypeString,
			Required: false,
		},
		{
			Name:     "Hours worked",
			Type:     InvoiceFieldTypeUint,
			Required: true,
		},
		{
			Name:     "Total cost",
			Type:     InvoiceFieldTypeUint,
			Required: true,
		},
	}
)
