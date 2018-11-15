package v1

type ErrorStatusT int
type InvoiceStatusT int
type UserManageActionT int
type InvoiceFieldTypeT int

const (
	// Error status codes
	ErrorStatusInvalid                        ErrorStatusT = 0
	ErrorStatusInvalidEmailOrPassword         ErrorStatusT = 1
	ErrorStatusMalformedEmail                 ErrorStatusT = 2
	ErrorStatusVerificationTokenInvalid       ErrorStatusT = 3
	ErrorStatusVerificationTokenExpired       ErrorStatusT = 4
	ErrorStatusVerificationTokenUnexpired     ErrorStatusT = 5
	ErrorStatusInvoiceNotFound                ErrorStatusT = 6
	ErrorStatusMalformedPassword              ErrorStatusT = 7
	ErrorStatusInvalidFileDigest              ErrorStatusT = 8
	ErrorStatusInvalidBase64                  ErrorStatusT = 9
	ErrorStatusInvalidMIMEType                ErrorStatusT = 10
	ErrorStatusUnsupportedMIMEType            ErrorStatusT = 11
	ErrorStatusInvalidInvoiceStatusTransition ErrorStatusT = 12
	ErrorStatusInvalidPublicKey               ErrorStatusT = 13
	ErrorStatusDuplicatePublicKey             ErrorStatusT = 14
	ErrorStatusNoPublicKey                    ErrorStatusT = 15
	ErrorStatusInvalidSignature               ErrorStatusT = 16
	ErrorStatusInvalidInput                   ErrorStatusT = 17
	ErrorStatusInvalidSigningKey              ErrorStatusT = 18
	ErrorStatusUserNotFound                   ErrorStatusT = 19
	ErrorStatusNotLoggedIn                    ErrorStatusT = 20
	ErrorStatusMalformedUsername              ErrorStatusT = 21
	ErrorStatusDuplicateUsername              ErrorStatusT = 22
	ErrorStatusUserLocked                     ErrorStatusT = 23
	ErrorStatusInvalidUserManageAction        ErrorStatusT = 24
	ErrorStatusMissingInvoiceFile             ErrorStatusT = 25
	ErrorStatusUserAlreadyExists              ErrorStatusT = 26
	ErrorStatusReasonNotProvided              ErrorStatusT = 27
	ErrorStatusMalformedInvoiceFile           ErrorStatusT = 28

	// Invoice status codes
	InvoiceStatusInvalid     InvoiceStatusT = 0 // Invalid status
	InvoiceStatusNotFound    InvoiceStatusT = 1 // Invoice not found
	InvoiceStatusNotReviewed InvoiceStatusT = 2 // Invoice has not been reviewed
	InvoiceStatusRejected    InvoiceStatusT = 3 // Invoice needs to be revised
	InvoiceStatusApproved    InvoiceStatusT = 4 // Invoice has been approved
	InvoiceStatusPaid        InvoiceStatusT = 5 // Invoice has been paid

	// User manage actions
	UserManageInvalid                              UserManageActionT = 0 // Invalid action type
	UserManageResendInvite                         UserManageActionT = 1
	UserManageRegenerateUpdateIdentityVerification UserManageActionT = 2
	UserManageUnlock                               UserManageActionT = 3
	UserManageLock                                 UserManageActionT = 4

	InvoiceFieldTypeInvalid InvoiceFieldTypeT = 0
	InvoiceFieldTypeString  InvoiceFieldTypeT = 1
	InvoiceFieldTypeUint    InvoiceFieldTypeT = 2
)

var (
	// ErrorStatus converts error status codes to human readable text.
	ErrorStatus = map[ErrorStatusT]string{
		ErrorStatusInvalid:                        "invalid status",
		ErrorStatusInvalidEmailOrPassword:         "invalid email or password",
		ErrorStatusMalformedEmail:                 "malformed email",
		ErrorStatusVerificationTokenInvalid:       "invalid verification token",
		ErrorStatusVerificationTokenExpired:       "expired verification token",
		ErrorStatusVerificationTokenUnexpired:     "verification token not yet expired",
		ErrorStatusInvoiceNotFound:                "invoice not found",
		ErrorStatusMalformedPassword:              "malformed password",
		ErrorStatusInvalidFileDigest:              "invalid file digest",
		ErrorStatusInvalidBase64:                  "invalid base64 file content",
		ErrorStatusInvalidMIMEType:                "invalid MIME type detected for file",
		ErrorStatusUnsupportedMIMEType:            "unsupported MIME type for file",
		ErrorStatusInvalidInvoiceStatusTransition: "invalid invoice status",
		ErrorStatusInvalidPublicKey:               "invalid public key",
		ErrorStatusNoPublicKey:                    "no active public key",
		ErrorStatusInvalidSignature:               "invalid signature",
		ErrorStatusInvalidInput:                   "invalid input",
		ErrorStatusInvalidSigningKey:              "invalid signing key",
		ErrorStatusUserNotFound:                   "user not found",
		ErrorStatusNotLoggedIn:                    "user not logged in",
		ErrorStatusMalformedUsername:              "malformed username",
		ErrorStatusDuplicateUsername:              "duplicate username",
		ErrorStatusUserLocked:                     "user locked due to too many login attempts",
		ErrorStatusInvalidUserManageAction:        "invalid user manage action",
		ErrorStatusMissingInvoiceFile:             "invoice file is missing",
		ErrorStatusUserAlreadyExists:              "user already exists",
		ErrorStatusReasonNotProvided:              "reason for action not provided",
		ErrorStatusMalformedInvoiceFile:           "malformed invoice file",
	}

	// InvoiceStatus converts propsal status codes to human readable text
	InvoiceStatus = map[InvoiceStatusT]string{
		InvoiceStatusInvalid:     "invalid invoice status",
		InvoiceStatusNotFound:    "not found",
		InvoiceStatusNotReviewed: "unreviewed",
		InvoiceStatusRejected:    "rejected",
		InvoiceStatusApproved:    "approved",
		InvoiceStatusPaid:        "paid",
	}

	// UserManageAction converts user manage actions to human readable text
	UserManageAction = map[UserManageActionT]string{
		UserManageInvalid:                              "invalid action",
		UserManageResendInvite:                         "resend invite",
		UserManageRegenerateUpdateIdentityVerification: "regenerate update identity verification",
		UserManageUnlock:                               "unlock user",
		UserManageLock:                                 "lock user",
	}
)
