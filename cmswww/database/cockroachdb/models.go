package cockroachdb

import (
	"database/sql"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/lib/pq"
)

const (
	tableNameUser           = "users"
	tableNameIdentity       = "identities"
	tableNameInvoice        = "invoices"
	tableNameInvoiceFile    = "invoice_files"
	tableNameInvoiceChange  = "invoice_changes"
	tableNameInvoicePayment = "invoice_payments"
)

type User struct {
	gorm.Model
	Email                                     string         `gorm:"type:varchar(100);unique_index"`
	Username                                  sql.NullString `gorm:"unique"`
	HashedPassword                            sql.NullString
	Name                                      string `gorm:"not_null"`
	Location                                  string `gorm:"not_null"`
	ExtendedPublicKey                         string `gorm:"not_null"`
	Admin                                     bool   `gorm:"not_null"`
	RegisterVerificationToken                 sql.NullString
	RegisterVerificationExpiry                pq.NullTime
	UpdateIdentityVerificationToken           sql.NullString
	UpdateIdentityVerificationExpiry          pq.NullTime
	ResetPasswordVerificationToken            sql.NullString
	ResetPasswordVerificationExpiry           pq.NullTime
	UpdateExtendedPublicKeyVerificationToken  sql.NullString
	UpdateExtendedPublicKeyVerificationExpiry pq.NullTime
	LastLogin                                 pq.NullTime
	FailedLoginAttempts                       uint64 `gorm:"not_null"`
	PaymentAddressIndex                       uint64 `gorm:"not_null"`
	EmailNotifications                        uint64 `gorm:"not_null"`

	Identities []Identity
	Invoices   []Invoice
}

func (u User) TableName() string {
	return tableNameUser
}

type Identity struct {
	gorm.Model
	UserID      uint           `gorm:"not_null"`
	Key         sql.NullString `gorm:"unique"`
	Activated   pq.NullTime
	Deactivated pq.NullTime
}

func (i Identity) TableName() string {
	return tableNameIdentity
}

type Invoice struct {
	Token              string    `gorm:"primary_key"`
	Version            string    `gorm:"primary_key"`
	UserID             uint      `gorm:"not_null"`
	Username           string    `gorm:"-"` // Only populated when reading from the database
	Month              uint      `gorm:"not_null"`
	Year               uint      `gorm:"not_null"`
	Timestamp          time.Time `gorm:"not_null"`
	Status             uint      `gorm:"not_null"`
	StatusChangeReason string
	PublicKey          string `gorm:"not_null"`
	UserSignature      string `gorm:"not_null"`
	ServerSignature    string `gorm:"not_null"`
	MerkleRoot         string `gorm:"not_null"`
	Proposal           string

	Files    []InvoiceFile    `gorm:"foreignkey:invoice_token,invoice_version;association_foreignkey:token,version"`
	Changes  []InvoiceChange  `gorm:"foreignkey:invoice_token,invoice_version;association_foreignkey:token,version"`
	Payments []InvoicePayment `gorm:"foreignkey:invoice_token,invoice_version;association_foreignkey:token,version"`

	// gorm.Model fields, included manually
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (i Invoice) TableName() string {
	return tableNameInvoice
}

type InvoiceFile struct {
	ID             int64  `gorm:"primary_key;auto_increment:false"`
	InvoiceToken   string `gorm:"primary_key"`
	InvoiceVersion string `gorm:"primary_key"`
	Name           string `gorm:"not_null"`
	MIME           string `gorm:"not_null"`
	Digest         string `gorm:"not_null"`
	Payload        string `gorm:"type:text"`
}

func (i InvoiceFile) TableName() string {
	return tableNameInvoiceFile
}

type InvoiceChange struct {
	InvoiceToken   string `gorm:"primary_key"`
	InvoiceVersion string `gorm:"primary_key"`
	AdminPublicKey string
	NewStatus      uint
	Timestamp      time.Time
}

func (i InvoiceChange) TableName() string {
	return tableNameInvoiceChange
}

type InvoicePayment struct {
	InvoiceToken   string `gorm:"primary_key"`
	InvoiceVersion string `gorm:"primary_key"`
	IsTotalCost    bool   `gorm:"not_null"`
	Address        string `gorm:"not_null"`
	Amount         uint   `gorm:"not_null"`
	TxNotBefore    int64  `gorm:"not_null"`
	PollExpiry     int64
	TxID           string
}

func (i InvoicePayment) TableName() string {
	return tableNameInvoicePayment
}
