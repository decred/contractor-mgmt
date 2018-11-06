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
	tableNameInvoiceChange  = "invoice_changes"
	tableNameInvoicePayment = "invoice_payments"
)

type User struct {
	gorm.Model
	Email                            string         `gorm:"type:varchar(100);unique_index"`
	Username                         sql.NullString `gorm:"unique"`
	HashedPassword                   sql.NullString
	Name                             string `gorm:"not_null"`
	Location                         string `gorm:"not_null"`
	ExtendedPublicKey                string `gorm:"not_null"`
	Admin                            bool   `gorm:"not_null"`
	RegisterVerificationToken        sql.NullString
	RegisterVerificationExpiry       pq.NullTime
	UpdateIdentityVerificationToken  sql.NullString
	UpdateIdentityVerificationExpiry pq.NullTime
	ResetPasswordVerificationToken   sql.NullString
	ResetPasswordVerificationExpiry  pq.NullTime
	LastLogin                        pq.NullTime
	FailedLoginAttempts              uint64 `gorm:"not_null"`
	PaymentAddressIndex              uint64 `gorm:"not_null"`

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
	Token           string    `gorm:"primary_key"`
	UserID          uint      `gorm:"not_null"`
	Username        string    `gorm:"-"` // Only populated when reading from the database
	Month           uint      `gorm:"not_null"`
	Year            uint      `gorm:"not_null"`
	Timestamp       time.Time `gorm:"not_null"`
	Status          uint      `gorm:"not_null"`
	FilePayload     string    `gorm:"type:text"`
	FileMIME        string
	FileDigest      string
	PublicKey       string `gorm:"not_null"`
	UserSignature   string `gorm:"not_null"`
	ServerSignature string `gorm:"not_null"`
	Proposal        string

	Changes  []InvoiceChange
	Payments []InvoicePayment

	// gorm.Model fields, included manually
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (i Invoice) TableName() string {
	return tableNameInvoice
}

type InvoiceChange struct {
	gorm.Model
	InvoiceToken   string
	AdminPublicKey string
	NewStatus      uint
	Timestamp      time.Time
}

func (i InvoiceChange) TableName() string {
	return tableNameInvoiceChange
}

type InvoicePayment struct {
	gorm.Model
	InvoiceToken string
	Address      string `gorm:"not_null"`
	Amount       uint   `gorm:"not_null"`
	TxNotBefore  int64  `gorm:"not_null"`
	PollExpiry   int64
	TxID         string
}

func (i InvoicePayment) TableName() string {
	return tableNameInvoicePayment
}
