package cockroachdb

import (
	"database/sql"

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
}

type Identity struct {
	gorm.Model
	UserID      uint
	Key         sql.NullString `gorm:"unique"`
	Activated   pq.NullTime
	Deactivated pq.NullTime
}

func (Identity) TableName() string {
	return "identity"
}
