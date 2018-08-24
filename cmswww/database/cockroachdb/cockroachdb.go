package cockroachdb

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/badoux/checkmail"
	"github.com/decred/contractor-mgmt/cmswww/database"
	_ "github.com/lib/pq"
)

const (
	LastUserIDKey = "lastuserid"
)

var (
	_ database.Database = (*cockroachdb)(nil)
)

// cockroachdb implements the database interface.
type cockroachdb struct {
	sync.RWMutex
	shutdown bool // Backend is shutdown
	db       *sql.DB
}

// Version contains the database version.
type Version struct {
	Version uint32 `json:"version"` // Database version
	Time    int64  `json:"time"`    // Time of record creation
}

// Store new user.
//
// UserNew satisfies the backend interface.
func (c *cockroachdb) UserNew(u *database.User) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	log.Debugf("UserNew: %v", u)

	if err := checkmail.ValidateFormat(u.Email); err != nil {
		return database.ErrInvalidEmail
	}

	args := EncodeUser(u, true)
	_, err := c.db.Exec(templateInsertUser, args...)
	return err
}

// UserGet returns a user record if found in the database.
//
// UserGet satisfies the backend interface.
func (c *cockroachdb) UserGetByEmail(email string) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	rows, err := c.db.Query(templateGetUserByEmail, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	user, err := DecodeUser(rows)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, database.ErrUserNotFound
	}
	return user, nil
}

// UserGetByUsername returns a user record given its username, if found in the database.
//
// UserGetByUsername satisfies the backend interface.
func (c *cockroachdb) UserGetByUsername(username string) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	rows, err := c.db.Query(templateGetUserByUsername, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	user, err := DecodeUser(rows)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, database.ErrUserNotFound
	}
	return user, nil
}

// UserGetById returns a user record given its id, if found in the database.
//
// UserGetById satisfies the backend interface.
func (c *cockroachdb) UserGetById(id uint64) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	rows, err := c.db.Query(templateGetUserByID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	user, err := DecodeUser(rows)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, database.ErrUserNotFound
	}
	return user, nil
}

// Update existing user.
//
// UserUpdate satisfies the backend interface.
func (c *cockroachdb) UserUpdate(u *database.User) error {
	c.Lock()
	defer c.Unlock()
	/*
		if c.shutdown {
			return database.ErrShutdown
		}

		log.Debugf("UserUpdate: %v", u.Email)

		// Make sure user already exists
		exists, err := c.userdb.Has([]byte(u.Email), nil)
		if err != nil {
			return err
		} else if !exists {
			return database.ErrUserNotFound
		}

		payload, err := EncodeUser(*u)
		if err != nil {
			return err
		}

		return c.userdb.Put([]byte(u.Email), payload, nil)
	*/
	return nil
}

// Update existing user.
//
// AllUsers satisfies the backend interface.
func (c *cockroachdb) AllUsers(callbackFn func(u *database.User)) error {
	c.Lock()
	defer c.Unlock()
	/*
		if c.shutdown {
			return database.ErrShutdown
		}

		log.Debugf("AllUsers\n")

		iter := c.userdb.NewIterator(nil, nil)
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()

			if !isUserRecord(string(key)) {
				continue
			}

			u, err := DecodeUser(value)
			if err != nil {
				return err
			}

			callbackFn(u)
		}
		iter.Release()

		return iter.Error()
	*/
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

	// TODO: add migration support & versioning:
	// https://github.com/golang-migrate/migrate#use-in-your-go-project
	db, err := sql.Open("postgres", dataSource)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: ", err)
	}

	if _, err := db.Exec(templateCreateUserTable); err != nil {
		return nil, fmt.Errorf("error creating user table: ", err)
	}

	cdb := cockroachdb{
		db: db,
	}
	return &cdb, nil
}
