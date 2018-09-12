package cockroachdb

import (
	"encoding/hex"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

// EncodeUser encodes a database.User instance into a cockroachdb User.
func EncodeUser(dbUser *database.User) *User {
	user := User{}

	user.ID = uint(dbUser.ID)
	user.Email = dbUser.Email
	user.Username.String = dbUser.Username
	user.Admin = dbUser.Admin
	user.FailedLoginAttempts = dbUser.FailedLoginAttempts

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
		Admin:               user.Admin,
		FailedLoginAttempts: user.FailedLoginAttempts,
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
