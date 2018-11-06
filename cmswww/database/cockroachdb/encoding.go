package cockroachdb

import (
	"encoding/hex"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

// EncodeUser encodes a database.User instance into a cockroachdb User.
func EncodeUser(dbUser *database.User) *User {
	user := User{}

	user.ID = uint(dbUser.ID)
	user.Email = dbUser.Email
	user.Name = dbUser.Name
	user.Location = dbUser.Location
	user.ExtendedPublicKey = dbUser.ExtendedPublicKey
	user.Admin = dbUser.Admin
	user.FailedLoginAttempts = dbUser.FailedLoginAttempts
	user.PaymentAddressIndex = dbUser.PaymentAddressIndex

	if len(dbUser.Username) > 0 {
		user.Username.Valid = true
		user.Username.String = dbUser.Username
	}

	if dbUser.HashedPassword != nil {
		user.HashedPassword.Valid = true
		user.HashedPassword.String = hex.EncodeToString(dbUser.HashedPassword)
	}

	if dbUser.RegisterVerificationToken != nil {
		user.RegisterVerificationToken.Valid = true
		user.RegisterVerificationToken.String = hex.EncodeToString(dbUser.RegisterVerificationToken)

		user.RegisterVerificationExpiry.Valid = true
		user.RegisterVerificationExpiry.Time = time.Unix(dbUser.RegisterVerificationExpiry, 0)
	}

	if dbUser.UpdateIdentityVerificationToken != nil {
		user.UpdateIdentityVerificationToken.Valid = true
		user.UpdateIdentityVerificationToken.String = hex.EncodeToString(dbUser.UpdateIdentityVerificationToken)

		user.UpdateIdentityVerificationExpiry.Valid = true
		user.UpdateIdentityVerificationExpiry.Time = time.Unix(dbUser.UpdateIdentityVerificationExpiry, 0)
	}

	if dbUser.ResetPasswordVerificationToken != nil {
		user.ResetPasswordVerificationToken.Valid = true
		user.ResetPasswordVerificationToken.String = hex.EncodeToString(dbUser.ResetPasswordVerificationToken)

		user.ResetPasswordVerificationExpiry.Valid = true
		user.ResetPasswordVerificationExpiry.Time = time.Unix(dbUser.ResetPasswordVerificationExpiry, 0)
	}

	if dbUser.LastLogin != 0 {
		user.LastLogin.Valid = true
		user.LastLogin.Time = time.Unix(dbUser.LastLogin, 0)
	}

	for _, dbId := range dbUser.Identities {
		user.Identities = append(user.Identities, *EncodeIdentity(&dbId))
	}

	return &user
}

// DecodeUser decodes a cockroachdb User instance into a generic database.User.
func DecodeUser(user *User) (*database.User, error) {
	dbUser := database.User{
		ID:                  uint64(user.ID),
		Email:               user.Email,
		Username:            user.Username.String,
		Name:                user.Name,
		Location:            user.Location,
		ExtendedPublicKey:   user.ExtendedPublicKey,
		Admin:               user.Admin,
		FailedLoginAttempts: user.FailedLoginAttempts,
		PaymentAddressIndex: user.PaymentAddressIndex,
	}

	var err error

	if len(user.HashedPassword.String) > 0 {
		dbUser.HashedPassword, err = hex.DecodeString(user.HashedPassword.String)
		if err != nil {
			return nil, err
		}
	}

	if user.RegisterVerificationToken.Valid {
		dbUser.RegisterVerificationToken, err = hex.DecodeString(user.RegisterVerificationToken.String)
		if err != nil {
			return nil, err
		}
		dbUser.RegisterVerificationExpiry = user.RegisterVerificationExpiry.Time.Unix()
	}

	if user.UpdateIdentityVerificationToken.Valid {
		dbUser.UpdateIdentityVerificationToken, err = hex.DecodeString(user.UpdateIdentityVerificationToken.String)
		if err != nil {
			return nil, err
		}
		dbUser.UpdateIdentityVerificationExpiry = user.UpdateIdentityVerificationExpiry.Time.Unix()
	}

	if user.ResetPasswordVerificationToken.Valid {
		dbUser.ResetPasswordVerificationToken, err = hex.DecodeString(user.ResetPasswordVerificationToken.String)
		if err != nil {
			return nil, err
		}
		dbUser.ResetPasswordVerificationExpiry = user.ResetPasswordVerificationExpiry.Time.Unix()
	}

	if user.LastLogin.Valid {
		dbUser.LastLogin = user.LastLogin.Time.Unix()
	}

	for _, id := range user.Identities {
		dbId, err := DecodeIdentity(&id)
		if err != nil {
			return nil, err
		}
		dbUser.Identities = append(dbUser.Identities, *dbId)
	}

	return &dbUser, nil
}

// EncodeIdentity encodes a generic database.Identity instance into a cockroachdb Identity.
func EncodeIdentity(dbId *database.Identity) *Identity {
	id := Identity{}

	id.ID = uint(dbId.ID)
	id.UserID = uint(dbId.UserID)

	if len(dbId.Key) != 0 {
		id.Key.Valid = true
		id.Key.String = hex.EncodeToString(dbId.Key[:])
	}

	if dbId.Activated != 0 {
		id.Activated.Valid = true
		id.Activated.Time = time.Unix(dbId.Activated, 0)
	}

	if dbId.Deactivated != 0 {
		id.Deactivated.Valid = true
		id.Deactivated.Time = time.Unix(dbId.Deactivated, 0)
	}

	return &id
}

// DecodeIdentity decodes a cockroachdb Identity instance into a generic database.Identity.
func DecodeIdentity(id *Identity) (*database.Identity, error) {
	dbId := database.Identity{}

	dbId.ID = uint64(id.ID)
	dbId.UserID = uint64(id.UserID)

	if id.Key.Valid {
		pk, err := hex.DecodeString(id.Key.String)
		if err != nil {
			return nil, err
		}

		copy(dbId.Key[:], pk)
	}

	if id.Activated.Valid {
		dbId.Activated = id.Activated.Time.Unix()
	}

	if id.Deactivated.Valid {
		dbId.Deactivated = id.Deactivated.Time.Unix()
	}

	return &dbId, nil
}

// EncodeInvoice encodes a generic database.Invoice instance into a cockroachdb
// Invoice.
func EncodeInvoice(dbInvoice *database.Invoice) *Invoice {
	invoice := Invoice{}

	invoice.Token = dbInvoice.Token
	invoice.UserID = uint(dbInvoice.UserID)
	invoice.Month = uint(dbInvoice.Month)
	invoice.Year = uint(dbInvoice.Year)
	invoice.Status = uint(dbInvoice.Status)
	invoice.Timestamp = time.Unix(dbInvoice.Timestamp, 0)
	if dbInvoice.File != nil {
		invoice.FilePayload = dbInvoice.File.Payload
		invoice.FileMIME = dbInvoice.File.MIME
		invoice.FileDigest = dbInvoice.File.Digest
	}
	invoice.PublicKey = dbInvoice.PublicKey
	invoice.UserSignature = dbInvoice.UserSignature
	invoice.ServerSignature = dbInvoice.ServerSignature
	invoice.Proposal = dbInvoice.Proposal

	for _, dbInvoiceChange := range dbInvoice.Changes {
		invoiceChange := EncodeInvoiceChange(&dbInvoiceChange)
		invoiceChange.InvoiceToken = invoice.Token
		invoice.Changes = append(invoice.Changes, *invoiceChange)
		invoice.Status = invoiceChange.NewStatus
	}

	for _, dbInvoicePayment := range dbInvoice.Payments {
		invoicePayment := EncodeInvoicePayment(&dbInvoicePayment)
		invoicePayment.InvoiceToken = invoice.Token
		invoice.Payments = append(invoice.Payments, *invoicePayment)
	}

	return &invoice
}

// EncodeInvoiceChange encodes a generic database.InvoiceChange instance into a cockroachdb
// InvoiceChange.
func EncodeInvoiceChange(dbInvoiceChange *database.InvoiceChange) *InvoiceChange {
	invoiceChange := InvoiceChange{}

	invoiceChange.AdminPublicKey = dbInvoiceChange.AdminPublicKey
	invoiceChange.NewStatus = uint(dbInvoiceChange.NewStatus)
	invoiceChange.Timestamp = time.Unix(dbInvoiceChange.Timestamp, 0)

	return &invoiceChange
}

// EncodeInvoicePayment encodes a generic database.InvoicePayment instance into a cockroachdb
// InvoicePayment.
func EncodeInvoicePayment(dbInvoicePayment *database.InvoicePayment) *InvoicePayment {
	invoicePayment := InvoicePayment{}

	invoicePayment.ID = uint(dbInvoicePayment.ID)
	invoicePayment.InvoiceToken = dbInvoicePayment.InvoiceToken
	invoicePayment.Address = dbInvoicePayment.Address
	invoicePayment.Amount = uint(dbInvoicePayment.Amount)
	invoicePayment.TxNotBefore = dbInvoicePayment.TxNotBefore
	invoicePayment.PollExpiry = dbInvoicePayment.PollExpiry
	invoicePayment.TxID = dbInvoicePayment.TxID

	return &invoicePayment
}

// DecodeInvoice decodes a cockroachdb Invoice instance into a generic database.Invoice.
func DecodeInvoice(invoice *Invoice) (*database.Invoice, error) {
	dbInvoice := database.Invoice{}

	dbInvoice.Token = invoice.Token
	dbInvoice.UserID = uint64(invoice.UserID)
	dbInvoice.Username = invoice.Username
	dbInvoice.Month = uint16(invoice.Month)
	dbInvoice.Year = uint16(invoice.Year)
	dbInvoice.Status = v1.InvoiceStatusT(invoice.Status)
	dbInvoice.Timestamp = invoice.Timestamp.Unix()
	if invoice.FilePayload != "" {
		dbInvoice.File = &database.File{
			Payload: invoice.FilePayload,
			MIME:    invoice.FileMIME,
			Digest:  invoice.FileDigest,
		}
	}
	dbInvoice.PublicKey = invoice.PublicKey
	dbInvoice.UserSignature = invoice.UserSignature
	dbInvoice.ServerSignature = invoice.ServerSignature
	dbInvoice.Proposal = invoice.Proposal
	/*
		for _, invoiceChange := range invoice.Changes {
			dbInvoiceChange := DecodeInvoiceChange(&invoiceChange)
			dbInvoice.Changes = append(dbInvoice.Changes, *dbInvoiceChange)
		}
	*/
	for _, invoicePayment := range invoice.Payments {
		dbInvoicePayment := DecodeInvoicePayment(&invoicePayment)
		dbInvoice.Payments = append(dbInvoice.Payments, *dbInvoicePayment)
	}

	return &dbInvoice, nil
}

// DecodeInvoiceChange decodes a cockroachdb InvoiceChange instance into a generic
// database.InvoiceChange.
func DecodeInvoiceChange(invoiceChange *InvoiceChange) *database.InvoiceChange {
	dbInvoiceChange := database.InvoiceChange{}

	dbInvoiceChange.AdminPublicKey = invoiceChange.AdminPublicKey
	dbInvoiceChange.NewStatus = v1.InvoiceStatusT(invoiceChange.NewStatus)
	dbInvoiceChange.Timestamp = invoiceChange.Timestamp.Unix()

	return &dbInvoiceChange
}

// DecodeInvoicePayment decodes a cockroachdb InvoicePayment instance into a
// generic database.InvoicePayment.
func DecodeInvoicePayment(invoicePayment *InvoicePayment) *database.InvoicePayment {
	dbInvoicePayment := database.InvoicePayment{}

	dbInvoicePayment.ID = uint64(invoicePayment.ID)
	dbInvoicePayment.InvoiceToken = invoicePayment.InvoiceToken
	dbInvoicePayment.Address = invoicePayment.Address
	dbInvoicePayment.Amount = uint64(invoicePayment.Amount)
	dbInvoicePayment.TxNotBefore = invoicePayment.TxNotBefore
	dbInvoicePayment.PollExpiry = invoicePayment.PollExpiry
	dbInvoicePayment.TxID = invoicePayment.TxID

	return &dbInvoicePayment
}

// DecodeInvoices decodes an array of cockroachdb Invoice instances into
// generic database.Invoices.
func DecodeInvoices(invoices []Invoice) ([]database.Invoice, error) {
	dbInvoices := make([]database.Invoice, 0, len(invoices))

	for _, invoice := range invoices {
		dbInvoice, err := DecodeInvoice(&invoice)
		if err != nil {
			return nil, err
		}

		dbInvoices = append(dbInvoices, *dbInvoice)
	}

	return dbInvoices, nil
}
