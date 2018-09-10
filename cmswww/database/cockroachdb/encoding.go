package cockroachdb

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/decred/contractor-mgmt/cmswww/database"
	"github.com/decred/politeia/politeiad/api/v1/identity"
)

func toVar(num int) string {
	return fmt.Sprintf("$%v", num)
}

func toExpr(variable string, num int) string {
	return fmt.Sprintf("%v = $%v", variable, num)
}

// EncodeUserForCreate converts a database.User into the pieces needed to execute
// the SQL statement to create the user.
func EncodeUserForCreate(u *User) (string, string, []interface{}, error) {
	var (
		variablesClause []string
		valuesClause    []string
		values          = make([]interface{}, 0)
	)

	variablesClause = append(variablesClause, "email")
	valuesClause = append(valuesClause, toVar(len(variablesClause)))
	values = append(values, u.email)

	if u.usernameModified {
		variablesClause = append(variablesClause, "username")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		values = append(values, u.username)
	}

	if u.hashedPasswordModified {
		variablesClause = append(variablesClause, "hashed_password")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		values = append(values, hex.EncodeToString(u.hashedPassword))
	}

	variablesClause = append(variablesClause, "admin")
	valuesClause = append(valuesClause, toVar(len(variablesClause)))
	values = append(values, u.admin)

	if u.registerVerificationModified {
		variablesClause = append(variablesClause, "register_verification_token")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		variablesClause = append(variablesClause, "register_verification_expiry")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))

		if len(u.registerVerificationToken) != 0 {
			values = append(values,
				hex.EncodeToString(u.registerVerificationToken),
				time.Unix(u.registerVerificationExpiry, 0),
			)
		} else {
			values = append(values,
				sql.NullString{},
				pq.NullTime{},
			)
		}
	}

	if u.updateIdentityVerificationModified {
		variablesClause = append(variablesClause, "update_identity_verification_token")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		variablesClause = append(variablesClause, "update_identity_verification_expiry")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))

		if u.updateIdentityVerificationToken != nil && len(u.updateIdentityVerificationToken) != 0 {
			values = append(values,
				hex.EncodeToString(u.updateIdentityVerificationToken),
				time.Unix(u.updateIdentityVerificationExpiry, 0),
			)
		} else {
			values = append(values,
				sql.NullString{},
				pq.NullTime{},
			)
		}
	}

	if u.lastLoginModified {
		variablesClause = append(variablesClause, "last_login")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		values = append(values, time.Unix(u.lastLogin, 0))
	}

	variablesClause = append(variablesClause, "failed_login_attempts")
	valuesClause = append(valuesClause, toVar(len(variablesClause)))
	values = append(values, u.failedLoginAttempts)

	variablesClauseStr := strings.Join(variablesClause, ", ")
	valuesClauseStr := strings.Join(valuesClause, ", ")
	return variablesClauseStr, valuesClauseStr, values, nil
}

// EncodeUserForUpdate converts a database.User the pieces needed to execute
// the SQL statement to update the user.
func EncodeUserForUpdate(u *User) (string, string, []interface{}, error) {
	var (
		expressions []string
		whereClause string
		values      = make([]interface{}, 0)
	)

	if u.emailModified {
		values = append(values, u.email)
		expressions = append(expressions, toExpr("email", len(values)))
	}

	if u.usernameModified {
		values = append(values, u.username)
		expressions = append(expressions, toExpr("username", len(values)))
	}

	if u.hashedPasswordModified {
		values = append(values, hex.EncodeToString(u.hashedPassword))
		expressions = append(expressions, toExpr("hashed_password", len(values)))
	}

	if u.adminModified {
		values = append(values, u.admin)
		expressions = append(expressions, toExpr("admin", len(values)))
	}

	if u.registerVerificationModified {
		if len(u.registerVerificationToken) != 0 {
			values = append(values, hex.EncodeToString(u.registerVerificationToken))
		} else {
			values = append(values, sql.NullString{})
		}
		expressions = append(expressions, toExpr("register_verification_token",
			len(values)))
		if len(u.registerVerificationToken) != 0 {
			values = append(values, time.Unix(u.registerVerificationExpiry, 0))
		} else {
			values = append(values, pq.NullTime{})
		}
		expressions = append(expressions, toExpr("register_verification_expiry",
			len(values)))
	}

	if u.updateIdentityVerificationModified {
		if u.updateIdentityVerificationToken != nil && len(u.updateIdentityVerificationToken) != 0 {
			values = append(values, hex.EncodeToString(u.updateIdentityVerificationToken))
		} else {
			values = append(values, sql.NullString{})
		}
		expressions = append(expressions,
			toExpr("update_identity_verification_token", len(values)))
		if u.updateIdentityVerificationToken != nil && len(u.updateIdentityVerificationToken) != 0 {
			values = append(values, time.Unix(u.updateIdentityVerificationExpiry, 0))
		} else {
			values = append(values, pq.NullTime{})
		}
		expressions = append(expressions,
			toExpr("update_identity_verification_expiry", len(values)))
	}

	if u.lastLoginModified {
		values = append(values, time.Unix(u.lastLogin, 0))
		expressions = append(expressions, toExpr("last_login", len(values)))
	}

	if u.failedLoginAttemptsModified {
		values = append(values, u.failedLoginAttempts)
		expressions = append(expressions, toExpr("failed_login_attempts",
			len(values)))
	}

	expressionsStr := strings.Join(expressions, ", ")

	values = append(values, u.id)
	whereClause = fmt.Sprintf("id = $%v", len(values))

	return expressionsStr, whereClause, values, nil
}

// DecodeUser decodes a sql.Rows row into a database.User instance.
func DecodeUser(rows *sql.Rows) (database.User, error) {
	if !rows.Next() {
		return nil, nil
	}

	username := sql.NullString{}
	hashedPassword := sql.NullString{}
	registerVerificationToken := sql.NullString{}
	registerVerificationExpiry := pq.NullTime{}
	updateIdentityVerificationToken := sql.NullString{}
	updateIdentityVerificationExpiry := pq.NullTime{}
	lastLogin := pq.NullTime{}

	u := User{}
	err := rows.Scan(
		&u.id,
		&u.email,
		&username,
		&hashedPassword,
		&u.admin,
		&registerVerificationToken,
		&registerVerificationExpiry,
		&updateIdentityVerificationToken,
		&updateIdentityVerificationExpiry,
		&lastLogin,
		&u.failedLoginAttempts,
	)
	if err != nil {
		return nil, err
	}

	if username.Valid {
		u.username = username.String
	}
	if hashedPassword.Valid {
		u.hashedPassword, _ = hex.DecodeString(hashedPassword.String)
	}

	if registerVerificationToken.Valid && registerVerificationToken.String != "" {
		u.registerVerificationToken, _ = hex.DecodeString(
			registerVerificationToken.String)
	}
	if registerVerificationExpiry.Valid {
		u.registerVerificationExpiry = registerVerificationExpiry.Time.Unix()
	}

	if updateIdentityVerificationToken.Valid &&
		updateIdentityVerificationToken.String != "" {
		u.updateIdentityVerificationToken, _ = hex.DecodeString(
			updateIdentityVerificationToken.String)
	}
	if updateIdentityVerificationExpiry.Valid {
		u.updateIdentityVerificationExpiry = updateIdentityVerificationExpiry.Time.Unix()
	}

	if lastLogin.Valid {
		u.lastLogin = lastLogin.Time.Unix()
	}

	return &u, err
}

// EncodeIdentityForCreate converts a database.Identity into the pieces needed
// to execute the SQL statement to create the identity.
func EncodeIdentityForCreate(userId uint64, id *Identity) (string, string, []interface{}) {
	var (
		variablesClause []string
		valuesClause    []string
		values          = make([]interface{}, 0)
	)

	variablesClause = append(variablesClause, "user_id")
	valuesClause = append(valuesClause, toVar(len(variablesClause)))
	values = append(values, userId)

	if id.keyModified {
		variablesClause = append(variablesClause, "key")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		values = append(values, id.EncodedKey())
	}

	if id.activatedModified {
		variablesClause = append(variablesClause, "activated")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		values = append(values, time.Unix(id.activated, 0))
	}

	if id.deactivatedModified {
		variablesClause = append(variablesClause, "deactivated")
		valuesClause = append(valuesClause, toVar(len(variablesClause)))
		values = append(values, time.Unix(id.deactivated, 0))
	}

	variablesClauseStr := strings.Join(variablesClause, ", ")
	valuesClauseStr := strings.Join(valuesClause, ", ")
	return variablesClauseStr, valuesClauseStr, values
}

// EncodeIdentityForUpdate converts a database.Identity into the pieces needed
// to execute the SQL statement to update the identity.
func EncodeIdentityForUpdate(userId uint64, id *Identity) (string, string, []interface{}) {
	var (
		expressions []string
		whereClause string
		values      = make([]interface{}, 0)
	)

	if id.activatedModified {
		values = append(values, time.Unix(id.activated, 0))
		expressions = append(expressions, toExpr("activated", len(values)))
	}

	if id.deactivatedModified {
		values = append(values, time.Unix(id.deactivated, 0))
		expressions = append(expressions, toExpr("deactivated", len(values)))
	}

	expressionsStr := strings.Join(expressions, ", ")

	values = append(values, userId)
	values = append(values, id.EncodedKey())
	whereClause = fmt.Sprintf("user_id = $%v and key = $%v", len(values)-1, len(values))

	return expressionsStr, whereClause, values
}

func EncodeIdentityForDelete(userId uint64, id *Identity) (string, []interface{}) {
	var (
		whereClause string
		values      = make([]interface{}, 0)
	)

	values = append(values, userId)
	values = append(values, id.EncodedKey())
	whereClause = fmt.Sprintf("user_id = $%v and key = $%v", len(values), len(values)+1)
	return whereClause, values
}

// DecodeIdentity decodes a sql.Rows row into a database.Identity instance.
func DecodeIdentity(rows *sql.Rows) (database.Identity, error) {
	if !rows.Next() {
		return nil, nil
	}

	var (
		encodedKey  string
		activated   pq.NullTime
		deactivated pq.NullTime
		id          Identity
	)

	err := rows.Scan(
		&id.userID,
		&encodedKey,
		&activated,
		&deactivated,
	)
	if err != nil {
		return nil, err
	}

	pk, err := hex.DecodeString(encodedKey)
	if err != nil {
		return nil, err
	}
	id.key = [identity.PublicKeySize]byte{}
	copy(id.key[:], pk)

	if activated.Valid {
		id.activated = activated.Time.Unix()
	}
	if deactivated.Valid {
		id.deactivated = deactivated.Time.Unix()
	}

	return &id, err
}
