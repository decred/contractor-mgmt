package cockroachdb

import (
	"database/sql"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/lib/pq"
)

type User struct {
	gorm.Model
	Email                            string         `gorm:"type:varchar(100);unique_index"`
	Username                         sql.NullString `gorm:"unique"`
	HashedPassword                   sql.NullString
	Admin                            bool `gorm:"not_null"`
	RegisterVerificationToken        sql.NullString
	RegisterVerificationExpiry       pq.NullTime
	UpdateIdentityVerificationToken  sql.NullString
	UpdateIdentityVerificationExpiry pq.NullTime
	LastLogin                        pq.NullTime
	FailedLoginAttempts              uint64 `gorm:"not_null"`

	Identities []Identity
	Invoices   []Invoice
}

type Identity struct {
	gorm.Model
	UserID      uint           `gorm:"not_null"`
	Key         sql.NullString `gorm:"unique"`
	Activated   pq.NullTime
	Deactivated pq.NullTime
}

type Invoice struct {
	Token           string `gorm:"primary_key"`
	UserID          uint   `gorm:"not_null"`
	Username        string `gorm:"-"` // Only populated when reading from the database
	Month           uint   `gorm:"not_null"`
	Year            uint   `gorm:"not_null"`
	FilePayload     string `gorm:"type:text"`
	FileMIME        string
	FileDigest      string
	PublicKey       string `gorm:"not_null"`
	UserSignature   string `gorm:"not_null"`
	ServerSignature string `gorm:"not_null"`

	Changes []InvoiceChange

	// gorm.Model fields, included manually
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type InvoiceChange struct {
	AdminPublicKey string    `gorm:"not_null"`
	NewStatus      uint      `gorm:"not_null"`
	Timestamp      time.Time `gorm:"not_null"`
}
