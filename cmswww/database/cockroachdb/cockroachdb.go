package cockroachdb

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/badoux/checkmail"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	_ database.Database = (*cockroachdb)(nil)
)

// cockroachdb implements the database interface.
type cockroachdb struct {
	sync.RWMutex
	shutdown bool // Backend is shutdown
	db       *gorm.DB
}

// Version contains the database version.
type Version struct {
	Version uint32 `json:"version"` // Database version
	Time    int64  `json:"time"`    // Time of record creation
}

/*
func (c *cockroachdb) getUserByQuery(query string, args ...interface{}) (database.User, error) {
	rows, err := c.query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dbu, err := DecodeUser(rows)
	if err != nil {
		return nil, err
	}

	if dbu == nil {
		return nil, database.ErrUserNotFound
	}

	rows, err = c.query(templateGetIdentitiesByUser, dbu.ID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for {
		dbid, err := DecodeIdentity(rows)
		if err != nil {
			return nil, err
		}

		if dbid == nil {
			break
		}

		u, _ := databaseUserToUser(dbu)
		id, _ := databaseIdentityToIdentity(dbid)
		u.identities = append(u.identities, id)
	}

	return dbu, nil
}
*/
// Store new user.
//
// NewUser satisfies the backend interface.
func (c *cockroachdb) NewUser(dbUser *database.User) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	user := EncodeUser(dbUser)

	log.Debugf("NewUser: %v", user.Email)

	if err := checkmail.ValidateFormat(user.Email); err != nil {
		return database.ErrInvalidEmail
	}

	c.db.Create(user)
	return nil
}

// Update an existing user.
//
// UpdateUser satisfies the backend interface.
func (c *cockroachdb) UpdateUser(dbUser *database.User) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	user := EncodeUser(dbUser)

	log.Debugf("UpdateUser: %v", user.Email)

	c.db.Save(user)
	return nil
}

// UserGet returns a user record if found in the database.
//
// UserGet satisfies the backend interface.
func (c *cockroachdb) GetUserByEmail(email string) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	var user User
	result := c.db.Where("email = ?", email).Preload("Identities").First(&user)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrUserNotFound
		}
		return nil, result.Error
	}

	return DecodeUser(&user)
}

// UserGetByUsername returns a user record given its username, if found in the database.
//
// UserGetByUsername satisfies the backend interface.
func (c *cockroachdb) GetUserByUsername(username string) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	var user User
	result := c.db.Where("username = ?", username).Preload("Identities").First(&user)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrUserNotFound
		}
		return nil, result.Error
	}

	return DecodeUser(&user)
}

// UserGetById returns a user record given its id, if found in the database.
//
// UserGetById satisfies the backend interface.
func (c *cockroachdb) GetUserById(id uint64) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	var user User
	result := c.db.Preload("Identities").First(&user, id)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrUserNotFound
		}
		return nil, result.Error
	}

	return DecodeUser(&user)
}

// Executes a callback on every user in the database.
//
// AllUsers satisfies the backend interface.
func (c *cockroachdb) AllUsers(callbackFn func(u *database.User)) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	log.Debugf("AllUsers\n")

	var users []User
	c.db.Find(&users)

	for _, user := range users {
		dbUser, err := DecodeUser(&user)
		if err != nil {
			return err
		}

		callbackFn(dbUser)
	}

	return nil
}

// Deletes all data from all tables.
//
// DeleteAllData satisfies the backend interface.
func (c *cockroachdb) DeleteAllData() error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	log.Debugf("DeleteAllData\n")

	c.db.DropTableIfExists(&User{}, &Identity{})
	return nil
}

// Close shuts down the database.  All interface functions MUST return with
// errShutdown if the backend is shutting down.
//
// Close satisfies the backend interface.
func (c *cockroachdb) Close() error {
	c.Lock()
	defer c.Unlock()

	c.shutdown = true
	return c.db.Close()
}

// New creates a new cockroachdb instance.
func New(dataDir, dbName, username, host string) (*cockroachdb, error) {
	log.Tracef("cockroachdb New")

	cockroachDBFile := filepath.Join(dataDir, "cockroachdb")

	v := url.Values{}
	v.Set("sslcert", filepath.Join(cockroachDBFile,
		fmt.Sprintf("client.%v.crt", username)))
	v.Set("sslkey", filepath.Join(cockroachDBFile,
		fmt.Sprintf("client.%v.key", username)))
	v.Set("sslmode", "verify-full")
	v.Set("sslrootcert", filepath.Join(cockroachDBFile, "ca.crt"))
	dataSource := fmt.Sprintf("postgresql://%v@%v/%v?%v", username, host,
		dbName, v.Encode())

	db, err := gorm.Open("postgres", dataSource)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	db.AutoMigrate(
		&User{},
		&Identity{},
	)

	cdb := cockroachdb{
		db: db,
	}
	return &cdb, nil
}
