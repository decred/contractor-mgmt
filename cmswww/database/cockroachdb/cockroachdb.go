package cockroachdb

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/badoux/checkmail"
	_ "github.com/lib/pq"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	_ database.Database = (*cockroachdb)(nil)
	_ database.User     = (*User)(nil)
	_ database.Identity = (*Identity)(nil)
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

func execute(db *sql.DB, query string, values ...interface{}) (sql.Result, error) {
	log.Debugf("Exec: %v\nValues: %v", query, values)
	return db.Exec(query, values...)
}

func (c *cockroachdb) execute(query string, values ...interface{}) (sql.Result, error) {
	return execute(c.db, query, values...)
}

func (c *cockroachdb) query(query string, values ...interface{}) (*sql.Rows, error) {
	log.Debugf("Query: %v\nValues: %v", query, values)
	return c.db.Query(query, values...)
}

func (c *cockroachdb) queryRow(query string, values ...interface{}) *sql.Row {
	log.Debugf("Query Row: %v\nValues: %v", query, values)
	return c.db.QueryRow(query, values...)
}

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

// Store new user.
//
// NewUser satisfies the backend interface.
func (c *cockroachdb) NewUser(dbu database.User) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	u, err := databaseUserToUser(dbu)
	if err != nil {
		return err
	}

	log.Debugf("NewUser: %v", u.email)

	if err := checkmail.ValidateFormat(u.email); err != nil {
		return database.ErrInvalidEmail
	}

	variablesClause, valuesClause, values, err := EncodeUserForCreate(u)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(templateInsertUser, variablesClause, valuesClause)
	row := c.queryRow(query, values...)
	err = row.Scan(&u.id)
	if err != nil {
		return err
	}

	for _, id := range u.identities {
		variablesClause, valuesClause, values = EncodeIdentityForCreate(u.id, id)
		query := fmt.Sprintf(templateInsertIdentity, variablesClause, valuesClause)
		_, err := c.execute(query, values...)
		if err != nil {
			return err
		}
	}

	u.resetModifiedFlags()
	return nil
}

// Update an existing user.
//
// UpdateUser satisfies the backend interface.
func (c *cockroachdb) UpdateUser(dbu database.User) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	u, err := databaseUserToUser(dbu)
	if err != nil {
		return err
	}

	log.Debugf("UpdateUser: %v", u.email)

	expressions, whereClause, values, err := EncodeUserForUpdate(u)
	if err != nil {
		return err
	}

	if len(expressions) > 0 {
		query := fmt.Sprintf(templateUpdateUser, expressions, whereClause)
		_, err = c.execute(query, values...)
		if err != nil {
			return err
		}
	}

	// Delete any removed identities.
	for _, id := range u.identitiesRemoved {
		// Delete the identity from the database.
		whereClause, values := EncodeIdentityForDelete(u.id, id)
		query := fmt.Sprintf(templateDeleteIdentity, whereClause)
		_, err := c.execute(query, values...)
		if err != nil {
			return err
		}

		// Remove the identity from the in-memory array.
		idx := getIdentityIdx(id, u.identities)
		if idx >= 0 {
			u.identities = remove(u.identities, idx)
		}
	}

	// Update any modified identities.
	for _, id := range u.identities {
		expressions, whereClause, values := EncodeIdentityForUpdate(u.id, id)
		if len(expressions) == 0 {
			continue
		}

		query := fmt.Sprintf(templateUpdateIdentity, expressions, whereClause)
		_, err = c.execute(query, values...)
		if err != nil {
			return err
		}
	}

	// Add any new identities.
	for _, id := range u.identitiesAdded {
		variablesClause, valuesClause, values := EncodeIdentityForCreate(u.id, id)
		query := fmt.Sprintf(templateInsertIdentity, variablesClause, valuesClause)
		_, err := c.execute(query, values...)
		if err != nil {
			return err
		}

		u.identities = append(u.identities, id)
	}

	u.resetModifiedFlags()
	return err
}

// UserGet returns a user record if found in the database.
//
// UserGet satisfies the backend interface.
func (c *cockroachdb) GetUserByEmail(email string) (database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	return c.getUserByQuery(templateGetUserByEmail, email)
}

// UserGetByUsername returns a user record given its username, if found in the database.
//
// UserGetByUsername satisfies the backend interface.
func (c *cockroachdb) GetUserByUsername(username string) (database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	return c.getUserByQuery(templateGetUserByUsername, username)
}

// UserGetById returns a user record given its id, if found in the database.
//
// UserGetById satisfies the backend interface.
func (c *cockroachdb) GetUserById(id uint64) (database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	return c.getUserByQuery(templateGetUserByID, id)
}

// Executes a callback on every user in the database.
//
// AllUsers satisfies the backend interface.
func (c *cockroachdb) AllUsers(callbackFn func(u database.User)) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	log.Debugf("AllUsers\n")

	rows, err := c.query(templateGetAllUsers)
	if err != nil {
		return err
	}
	defer rows.Close()

	for {
		user, err := DecodeUser(rows)
		if err != nil {
			return err
		}

		if user == nil {
			break
		}

		callbackFn(user)
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

	_, err := c.execute(templateDropIdentityTable)
	_, err = c.execute(templateDropUsersTable)
	return err
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

func createSchema(db *sql.DB) error {
	if _, err := execute(db, templateCreateUserTable); err != nil {
		return fmt.Errorf("error connecting to the database: %v", err)
	}

	if _, err := execute(db, templateCreateIdentityTable); err != nil {
		return fmt.Errorf("error connecting to the database: %v", err)
	}

	return nil
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

	// TODO: add migration support & versioning?
	// https://github.com/golang-migrate/migrate#use-in-your-go-project
	db, err := sql.Open("postgres", dataSource)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	err = createSchema(db)
	if err != nil {
		return nil, err
	}

	cdb := cockroachdb{
		db: db,
	}
	return &cdb, nil
}
