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
	user.EmailNotifications = dbUser.EmailNotifications

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

	if dbUser.UpdateExtendedPublicKeyVerificationToken != nil {
		user.UpdateExtendedPublicKeyVerificationToken.Valid = true
		user.UpdateExtendedPublicKeyVerificationToken.String = hex.EncodeToString(dbUser.UpdateExtendedPublicKeyVerificationToken)

		user.UpdateExtendedPublicKeyVerificationExpiry.Valid = true
		user.UpdateExtendedPublicKeyVerificationExpiry.Time = time.Unix(dbUser.UpdateExtendedPublicKeyVerificationExpiry, 0)
	}

	if dbUser.LastLogin != 0 {
		user.LastLogin.Valid = true
		user.LastLogin.Time = time.Unix(dbUser.LastLogin, 0)
	}

	for _, dbID := range dbUser.Identities {
		user.Identities = append(user.Identities, *EncodeIdentity(&dbID))
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
		EmailNotifications:  user.EmailNotifications,
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

	if user.UpdateExtendedPublicKeyVerificationToken.Valid {
		dbUser.UpdateExtendedPublicKeyVerificationToken, err = hex.DecodeString(user.UpdateExtendedPublicKeyVerificationToken.String)
		if err != nil {
			return nil, err
		}
		dbUser.UpdateExtendedPublicKeyVerificationExpiry = user.UpdateExtendedPublicKeyVerificationExpiry.Time.Unix()
	}

	if user.LastLogin.Valid {
		dbUser.LastLogin = user.LastLogin.Time.Unix()
	}

	for _, id := range user.Identities {
		dbID, err := DecodeIdentity(&id)
		if err != nil {
			return nil, err
		}
		dbUser.Identities = append(dbUser.Identities, *dbID)
	}

	return &dbUser, nil
}

// EncodeIdentity encodes a generic database.Identity instance into a cockroachdb Identity.
func EncodeIdentity(dbID *database.Identity) *Identity {
	id := Identity{}

	id.ID = uint(dbID.ID)
	id.UserID = uint(dbID.UserID)

	if len(dbID.Key) != 0 {
		id.Key.Valid = true
		id.Key.String = hex.EncodeToString(dbID.Key[:])
	}

	if dbID.Activated != 0 {
		id.Activated.Valid = true
		id.Activated.Time = time.Unix(dbID.Activated, 0)
	}

	if dbID.Deactivated != 0 {
		id.Deactivated.Valid = true
		id.Deactivated.Time = time.Unix(dbID.Deactivated, 0)
	}

	return &id
}

// DecodeIdentity decodes a cockroachdb Identity instance into a generic database.Identity.
func DecodeIdentity(id *Identity) (*database.Identity, error) {
	dbID := database.Identity{}

	dbID.ID = uint64(id.ID)
	dbID.UserID = uint64(id.UserID)

	if id.Key.Valid {
		pk, err := hex.DecodeString(id.Key.String)
		if err != nil {
			return nil, err
		}

		copy(dbID.Key[:], pk)
	}

	if id.Activated.Valid {
		dbID.Activated = id.Activated.Time.Unix()
	}

	if id.Deactivated.Valid {
		dbID.Deactivated = id.Deactivated.Time.Unix()
	}

	return &dbID, nil
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
	invoice.StatusChangeReason = dbInvoice.StatusChangeReason
	invoice.Timestamp = time.Unix(dbInvoice.Timestamp, 0)
	invoice.PublicKey = dbInvoice.PublicKey
	invoice.UserSignature = dbInvoice.UserSignature
	invoice.ServerSignature = dbInvoice.ServerSignature
	invoice.MerkleRoot = dbInvoice.MerkleRoot
	invoice.Proposal = dbInvoice.Proposal
	invoice.Version = dbInvoice.Version

	for idx, dbInvoiceFile := range dbInvoice.Files {
		invoiceFile := EncodeInvoiceFile(&dbInvoiceFile)

		// Start the ID at 1 because gorm thinks it's a blank field if 0 is
		// passed and will automatically derive a value for it.
		invoiceFile.ID = int64(idx + 1)

		invoiceFile.InvoiceToken = invoice.Token
		invoiceFile.InvoiceVersion = invoice.Version
		invoice.Files = append(invoice.Files, *invoiceFile)
	}

	for _, dbInvoiceChange := range dbInvoice.Changes {
		invoiceChange := EncodeInvoiceChange(&dbInvoiceChange)
		invoiceChange.InvoiceToken = invoice.Token
		invoiceChange.InvoiceVersion = invoice.Version
		invoice.Changes = append(invoice.Changes, *invoiceChange)
		invoice.Status = invoiceChange.NewStatus
	}

	for _, dbInvoicePayment := range dbInvoice.Payments {
		invoicePayment := EncodeInvoicePayment(&dbInvoicePayment)
		invoicePayment.InvoiceToken = invoice.Token
		invoicePayment.InvoiceVersion = invoice.Version
		invoice.Payments = append(invoice.Payments, *invoicePayment)
	}

	return &invoice
}

// EncodeInvoiceFile encodes a generic database.InvoiceFile instance into a cockroachdb
// InvoiceFile.
func EncodeInvoiceFile(dbInvoiceFile *database.InvoiceFile) *InvoiceFile {
	invoiceFile := InvoiceFile{}

	invoiceFile.Name = dbInvoiceFile.Name
	invoiceFile.MIME = dbInvoiceFile.MIME
	invoiceFile.Digest = dbInvoiceFile.Digest
	invoiceFile.Payload = dbInvoiceFile.Payload

	return &invoiceFile
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

	invoicePayment.IsTotalCost = dbInvoicePayment.IsTotalCost
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
	dbInvoice.StatusChangeReason = invoice.StatusChangeReason
	dbInvoice.Timestamp = invoice.Timestamp.Unix()
	dbInvoice.PublicKey = invoice.PublicKey
	dbInvoice.UserSignature = invoice.UserSignature
	dbInvoice.ServerSignature = invoice.ServerSignature
	dbInvoice.MerkleRoot = invoice.MerkleRoot
	dbInvoice.Proposal = invoice.Proposal
	dbInvoice.Version = invoice.Version

	for _, invoiceFile := range invoice.Files {
		dbInvoiceFile := DecodeInvoiceFile(&invoiceFile)
		dbInvoice.Files = append(dbInvoice.Files, *dbInvoiceFile)
	}
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

// DecodeInvoiceFile decodes a cockroachdb InvoiceFile instance into a generic
// database.InvoiceFile.
func DecodeInvoiceFile(invoiceFile *InvoiceFile) *database.InvoiceFile {
	dbInvoiceFile := database.InvoiceFile{}

	dbInvoiceFile.Name = invoiceFile.Name
	dbInvoiceFile.MIME = invoiceFile.MIME
	dbInvoiceFile.Digest = invoiceFile.Digest
	dbInvoiceFile.Payload = invoiceFile.Payload

	return &dbInvoiceFile
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

	//dbInvoicePayment.ID = uint64(invoicePayment.ID)
	dbInvoicePayment.IsTotalCost = invoicePayment.IsTotalCost
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
