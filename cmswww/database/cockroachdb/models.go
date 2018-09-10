package cockroachdb

import (
	"encoding/hex"

	"github.com/decred/politeia/politeiad/api/v1/identity"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

type User struct {
	id                                 uint64
	idModified                         bool
	email                              string
	emailModified                      bool
	username                           string
	usernameModified                   bool
	hashedPassword                     []byte
	hashedPasswordModified             bool
	admin                              bool
	adminModified                      bool
	registerVerificationToken          []byte
	registerVerificationExpiry         int64
	registerVerificationModified       bool
	updateIdentityVerificationToken    []byte
	updateIdentityVerificationExpiry   int64
	updateIdentityVerificationModified bool
	lastLogin                          int64
	lastLoginModified                  bool
	failedLoginAttempts                uint64
	failedLoginAttemptsModified        bool

	identities        []*Identity
	identitiesAdded   []*Identity
	identitiesRemoved []*Identity
}

func (u *User) ID() uint64 {
	return u.id
}
func (u *User) SetID(id uint64) {
	u.id = id
	u.idModified = true
}

func (u *User) Email() string {
	return u.email
}
func (u *User) SetEmail(email string) {
	u.email = email
	u.emailModified = true
}

func (u *User) Username() string {
	return u.username
}
func (u *User) SetUsername(username string) {
	u.username = username
	u.usernameModified = true
}

func (u *User) HashedPassword() []byte {
	return u.hashedPassword
}
func (u *User) SetHashedPassword(hashedPassword []byte) {
	u.hashedPassword = hashedPassword
	u.hashedPasswordModified = true
}

func (u *User) Admin() bool {
	return u.admin
}
func (u *User) SetAdmin(admin bool) {
	u.admin = admin
	u.adminModified = true
}

func (u *User) RegisterVerificationToken() []byte {
	return u.registerVerificationToken
}
func (u *User) RegisterVerificationExpiry() int64 {
	return u.registerVerificationExpiry
}
func (u *User) SetRegisterVerificationTokenAndExpiry(token []byte, expiry int64) {
	u.registerVerificationToken, u.registerVerificationExpiry = token, expiry
	u.registerVerificationModified = true
}

func (u *User) UpdateIdentityVerificationToken() []byte {
	return u.updateIdentityVerificationToken
}
func (u *User) UpdateIdentityVerificationExpiry() int64 {
	return u.updateIdentityVerificationExpiry
}
func (u *User) SetUpdateIdentityVerificationTokenAndExpiry(token []byte, expiry int64) {
	u.updateIdentityVerificationToken, u.updateIdentityVerificationExpiry = token, expiry
	u.updateIdentityVerificationModified = true
}

func (u *User) LastLogin() int64 {
	return u.lastLogin
}
func (u *User) SetLastLogin(lastLogin int64) {
	u.lastLogin = lastLogin
	u.lastLoginModified = true
}

func (u *User) FailedLoginAttempts() uint64 {
	return u.failedLoginAttempts
}
func (u *User) SetFailedLoginAttempts(failedLoginAttempts uint64) {
	u.failedLoginAttempts = failedLoginAttempts
	u.failedLoginAttemptsModified = true
}

func (u *User) AddIdentity(dbid database.Identity) {
	id, err := databaseIdentityToIdentity(dbid)
	if err != nil {
		log.Error(err)
	}

	u.identitiesAdded = append(u.identitiesAdded, id)
}

func (u *User) RemoveIdentity(dbid database.Identity) {
	id, err := databaseIdentityToIdentity(dbid)
	if err != nil {
		log.Error(err)
	}

	idx := getIdentityIdx(id, u.identitiesAdded)
	if idx >= 0 {
		u.identitiesAdded = remove(u.identitiesAdded, idx)
		return
	}
	u.identitiesRemoved = append(u.identitiesRemoved, id)
}

func (u *User) Identities() []database.Identity {
	return identitiesToDatabaseIdentities(u.identities)
}

func (u *User) MostRecentIdentity() database.Identity {
	return u.identities[len(u.identities)-1]
}

func (u *User) resetModifiedFlags() {
	u.idModified = false
	u.emailModified = false
	u.usernameModified = false
	u.hashedPasswordModified = false
	u.adminModified = false
	u.registerVerificationModified = false
	u.updateIdentityVerificationModified = false
	u.lastLoginModified = false
	u.failedLoginAttemptsModified = false

	for _, id := range u.identities {
		id.resetModifiedFlags()
	}
	u.identitiesAdded = make([]*Identity, 0)
	u.identitiesRemoved = make([]*Identity, 0)
}

type Identity struct {
	userID              uint64                       `db:"user_id"`
	key                 [identity.PublicKeySize]byte `db:"key"`
	keyModified         bool
	activated           int64 `db:"activated"`
	activatedModified   bool
	deactivated         int64 `db:"deactivated"`
	deactivatedModified bool
}

func (id *Identity) Key() [identity.PublicKeySize]byte {
	return id.key
}
func (id *Identity) SetKey(key [identity.PublicKeySize]byte) {
	id.key = key
	id.keyModified = true
}

func (id *Identity) Activated() int64 {
	return id.activated
}
func (id *Identity) SetActivated(activated int64) {
	id.activated = activated
	id.activatedModified = true
}

func (id *Identity) Deactivated() int64 {
	return id.deactivated
}
func (id *Identity) SetDeactivated(deactivated int64) {
	id.deactivated = deactivated
	id.deactivatedModified = true
}

func (id *Identity) EncodedKey() string {
	return hex.EncodeToString(id.key[:])
}

func (id *Identity) IsActive() bool {
	return id.activated != 0 && id.deactivated == 0
}

func (id *Identity) resetModifiedFlags() {
	id.keyModified = false
	id.activatedModified = false
	id.deactivatedModified = false
}
