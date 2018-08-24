package cockroachdb

import (
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/lib/pq"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

// EncodeUser encodes User into a slice of different types that can be
// appended to a SQL query call.
func EncodeUser(u *database.User, encodeForCreation bool) []interface{} {
	args := make([]interface{}, 0)
	if !encodeForCreation {
		args = append(args, u.ID)
	}
	args = append(args,
		u.Email,
		u.Username,
		u.HashedPassword,
		u.Admin,
	)
	if len(u.RegisterVerificationToken) != 0 {
		args = append(args,
			u.RegisterVerificationToken,
			time.Unix(u.RegisterVerificationExpiry, 0),
		)
	} else {
		args = append(args,
			sql.NullString{},
			pq.NullTime{},
		)
	}
	if len(u.UpdateIdentityVerificationToken) != 0 {
		args = append(args,
			u.UpdateIdentityVerificationToken,
			time.Unix(u.UpdateIdentityVerificationExpiry, 0),
		)
	} else {
		args = append(args,
			sql.NullString{},
			pq.NullTime{},
		)
	}
	if !encodeForCreation {
		args = append(args, u.LastLogin)
	}
	args = append(args, u.FailedLoginAttempts)

	return args
}

// DecodeUser decodes a SQL row into a User.
func DecodeUser(rows *sql.Rows) (*database.User, error) {
	if !rows.Next() {
		return nil, nil
	}

	var u database.User
	var registerVerificationToken sql.NullString
	var registerVerificationExpiry pq.NullTime
	var updateIdentityVerificationToken sql.NullString
	var updateIdentityVerificationExpiry pq.NullTime
	var lastLogin pq.NullTime

	err := rows.Scan(
		&u.ID,
		&u.Email,
		&u.Username,
		&u.HashedPassword,
		&u.Admin,
		&registerVerificationToken,
		&registerVerificationExpiry,
		&updateIdentityVerificationToken,
		&updateIdentityVerificationExpiry,
		&lastLogin,
		&u.FailedLoginAttempts,
	)
	if err != nil {
		return nil, err
	}

	if registerVerificationToken.Valid {
		token, err := hex.DecodeString(registerVerificationToken.String)
		if err != nil {
			return nil, err
		}
		u.RegisterVerificationToken = token
		u.RegisterVerificationExpiry = registerVerificationExpiry.Time.Unix()
	}

	if updateIdentityVerificationToken.Valid {
		token, err := hex.DecodeString(updateIdentityVerificationToken.String)
		if err != nil {
			return nil, err
		}
		u.UpdateIdentityVerificationToken = token
		u.UpdateIdentityVerificationExpiry = updateIdentityVerificationExpiry.Time.Unix()
	}

	return &u, nil
}
