package cockroachdb

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/badoux/checkmail"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	_ database.Database = (*cockroachdb)(nil)
)

// cockroachdb implements the database interface.
type cockroachdb struct {
	db *gorm.DB
}

// Version contains the database version.
type Version struct {
	Version uint32 `json:"version"` // Database version
	Time    int64  `json:"time"`    // Time of record creation
}

func (c *cockroachdb) addWhereClause(db *gorm.DB, paramsMap map[string]interface{}) *gorm.DB {
	for k, v := range paramsMap {
		_, ok := v.([]uint)
		if ok {
			db = db.Where(k+" in ( ? )", v)
		} else {
			db = db.Where(k+" = ?", v)
		}
	}
	return db
}

func (c *cockroachdb) fetchInvoiceFiles(invoice *Invoice) error {
	result := c.db.Where("invoice_token = ? and invoice_version = ?",
		invoice.Token, invoice.Version).Find(&invoice.Files)
	return result.Error
}

func (c *cockroachdb) fetchInvoicePayments(invoice *Invoice) error {
	result := c.db.Where("invoice_token = ? and invoice_version = ?",
		invoice.Token, invoice.Version).Find(&invoice.Payments)
	return result.Error
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func (c *cockroachdb) dropTable(tableName string) error {
	b := make([]byte, 4)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	newTableName := tableName + string(b)

	result := c.db.Exec(fmt.Sprintf("ALTER TABLE IF EXISTS %v RENAME TO %v;",
		tableName, newTableName))
	if result.Error != nil {
		return result.Error
	}

	result = c.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %v CASCADE;",
		newTableName))
	return result.Error
}

// Store new user.
//
// CreateUser satisfies the backend interface.
func (c *cockroachdb) CreateUser(dbUser *database.User) error {
	user := EncodeUser(dbUser)
	log.Debugf("NewUser: %v", user.Email)

	if err := checkmail.ValidateFormat(user.Email); err != nil {
		return database.ErrInvalidEmail
	}

	return c.db.Create(user).Error
}

// Update an existing user.
//
// UpdateUser satisfies the backend interface.
func (c *cockroachdb) UpdateUser(dbUser *database.User) error {
	user := EncodeUser(dbUser)
	log.Debugf("UpdateUser: %v", user.Email)
	return c.db.Model(&User{}).Updates(*user).Error
}

// GetUser returns a user record if found in the database.
//
// GetUser satisfies the backend interface.
func (c *cockroachdb) GetUserByEmail(email string) (*database.User, error) {
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

// GetUserByUsername returns a user record given its username, if found in the database.
//
// GetUserByUsername satisfies the backend interface.
func (c *cockroachdb) GetUserByUsername(username string) (*database.User, error) {
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

// GetUserById returns a user record given its id, if found in the database.
//
// GetUserById satisfies the backend interface.
func (c *cockroachdb) GetUserById(id uint64) (*database.User, error) {
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

// GetUserIdByPublicKey returns a user record given its id, if found in the database.
//
// GetUserIdByPublicKey satisfies the backend interface.
func (c *cockroachdb) GetUserIdByPublicKey(publicKey string) (uint64, error) {
	var id Identity
	result := c.db.Where("key = ?", publicKey).First(&id)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return 0, database.ErrUserNotFound
		}
		return 0, result.Error
	}

	return uint64(id.UserID), nil
}

// Executes a callback on every user in the database.
//
// GetAllUsers satisfies the backend interface.
func (c *cockroachdb) GetAllUsers(callbackFn func(u *database.User)) error {
	log.Debugf("GetAllUsers")

	var users []User
	result := c.db.Find(&users)
	if result.Error != nil {
		return result.Error
	}

	for _, user := range users {
		dbUser, err := DecodeUser(&user)
		if err != nil {
			return err
		}

		callbackFn(dbUser)
	}

	return nil
}

// Returns a list of users and the total count that match the provided username.
//
// GetUsers satisfies the backend interface.
func (c *cockroachdb) GetUsers(username string, page int) ([]database.User, int, error) {
	log.Debugf("GetUsers")

	query := "? = '' OR (lower(username) like lower(?) || '%')"
	order := "created_at ASC"
	username = strings.TrimSpace(username)

	var users []User

	db := c.db
	if page > -1 {
		db = db.Offset(page * v1.ListPageSize).Limit(v1.ListPageSize)
	}

	result := db.Order(order).Find(&users, query, username, username)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	dbUsers := make([]database.User, 0, len(users))
	for _, user := range users {
		dbUser, err := DecodeUser(&user)
		if err != nil {
			return nil, 0, err
		}
		dbUsers = append(dbUsers, *dbUser)
	}

	// If the number of users returned equals the apage size,
	// find the count of all users that match the query.
	numMatches := len(users)
	if len(users) == v1.ListPageSize {
		result = c.db.Model(&User{}).Where(query, username,
			username).Count(&numMatches)
		if result.Error != nil {
			return nil, 0, result.Error
		}
	}

	return dbUsers, numMatches, nil
}

// Create a new invoice version.
//
// CreateInvoice satisfies the backend interface.
func (c *cockroachdb) CreateInvoice(dbInvoice *database.Invoice) error {
	invoice := EncodeInvoice(dbInvoice)

	log.Debugf("CreateInvoice: %v %v", invoice.Token, len(dbInvoice.Files))
	return c.db.Create(invoice).Error
}

// Update an existing invoice version.
//
// UpdateInvoice satisfies the backend interface.
func (c *cockroachdb) UpdateInvoice(dbInvoice *database.Invoice) error {
	invoice := EncodeInvoice(dbInvoice)

	log.Debugf("UpdateInvoice: %v", invoice.Token)

	return c.db.Save(invoice).Error
}

// Return the latest invoice version given its token.
//
// GetInvoiceByToken satisfies the backend interface.
func (c *cockroachdb) GetInvoiceByToken(token string) (*database.Invoice, error) {
	log.Debugf("GetInvoiceByToken: %v", token)

	tbl := fmt.Sprintf("%v i", tableNameInvoice)
	sel := "i.*, u.username"
	joins := fmt.Sprintf(
		"inner join %v u on i.user_id = u.id "+
			"inner join ("+
			"select token, max(version) version from %v group by token"+
			") i2 "+
			"on ("+
			"i2.token = i.token and i2.version = i.version"+
			")",
		tableNameUser, tableNameInvoice)

	var invoice Invoice
	result := c.db.Table(tbl).Select(sel).Joins(joins).Where(
		"i.token = ?", token).Scan(&invoice)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrInvoiceNotFound
		}
		return nil, result.Error
	}

	err := c.fetchInvoiceFiles(&invoice)
	if err != nil {
		return nil, err
	}

	err = c.fetchInvoicePayments(&invoice)
	if err != nil {
		return nil, err
	}

	return DecodeInvoice(&invoice)
}

// Return a list of the latest invoices.
//
// GetInvoices satisfies the backend interface.
func (c *cockroachdb) GetInvoices(invoicesRequest database.InvoicesRequest) ([]database.Invoice, int, error) {
	log.Debugf("GetInvoices")

	paramsMap := make(map[string]interface{})
	var err error
	if invoicesRequest.UserID != "" {
		paramsMap["i.user_id"], err = strconv.ParseUint(invoicesRequest.UserID, 10, 64)
		if err != nil {
			return nil, 0, err
		}
	}

	if invoicesRequest.StatusMap != nil && len(invoicesRequest.StatusMap) > 0 {
		statuses := make([]uint, 0, len(invoicesRequest.StatusMap))
		for k := range invoicesRequest.StatusMap {
			statuses = append(statuses, uint(k))
		}
		paramsMap["i.status"] = statuses
	}

	if invoicesRequest.Month != 0 {
		paramsMap["i.month"] = invoicesRequest.Month
	}

	if invoicesRequest.Year != 0 {
		paramsMap["i.year"] = invoicesRequest.Year
	}

	tbl := fmt.Sprintf("%v i", tableNameInvoice)
	sel := "i.*, u.username"
	joins := fmt.Sprintf(
		"inner join %v u on i.user_id = u.id "+
			"inner join ("+
			"select token, max(version) version from %v group by token"+
			") i2 "+
			"on ("+
			"i2.token = i.token and i2.version = i.version"+
			")",
		tableNameUser, tableNameInvoice)
	order := "i.timestamp asc"

	db := c.db.Table(tbl)
	if invoicesRequest.Page > -1 {
		offset := invoicesRequest.Page * v1.ListPageSize
		db = db.Offset(offset).Limit(v1.ListPageSize)
	}
	db = db.Select(sel).Joins(joins)
	db = c.addWhereClause(db, paramsMap)
	db = db.Order(order)

	var invoices []Invoice
	result := db.Scan(&invoices)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, 0, database.ErrInvoiceNotFound
		}
		return nil, 0, result.Error
	}

	// Set the invoice payments and files on each invoice.
	if invoicesRequest.IncludePayments || invoicesRequest.IncludeFiles {
		for idx := range invoices {
			if invoicesRequest.IncludeFiles {
				err = c.fetchInvoiceFiles(&invoices[idx])
				if err != nil {
					return nil, 0, err
				}
			}

			if invoicesRequest.IncludePayments {
				err = c.fetchInvoicePayments(&invoices[idx])
				if err != nil {
					return nil, 0, err
				}
			}
		}
	}

	// If the number of users returned equals the page size,
	// find the count of all users that match the query.
	numMatches := len(invoices)
	if len(invoices) == v1.ListPageSize {
		db = c.db.Table(tbl).Select(sel).Joins(joins)
		db = c.addWhereClause(db, paramsMap)
		result = db.Count(&numMatches)
		if result.Error != nil {
			return nil, 0, result.Error
		}
	}

	dbInvoices, err := DecodeInvoices(invoices)
	if err != nil {
		return nil, 0, err
	}
	return dbInvoices, numMatches, nil
}

// Update an existing invoice's payment.
//
// UpdateInvoicePayment satisfies the backend interface.
func (c *cockroachdb) UpdateInvoicePayment(
	token, version string,
	dbInvoicePayment *database.InvoicePayment,
) error {
	invoicePayment := EncodeInvoicePayment(dbInvoicePayment)
	invoicePayment.InvoiceToken = token
	invoicePayment.InvoiceVersion = version

	log.Debugf("UpdateInvoicePayment: %v", invoicePayment.InvoiceToken)

	return c.db.Save(invoicePayment).Error
}

// Create files for an invoice version.
//
// CreateInvoiceFiles satisfies the backend interface.
func (c *cockroachdb) CreateInvoiceFiles(
	token, version string,
	dbInvoiceFiles []database.InvoiceFile,
) error {
	log.Debugf("CreateInvoiceFiles: %v", token)

	for idx, dbInvoiceFile := range dbInvoiceFiles {
		invoiceFile := EncodeInvoiceFile(&dbInvoiceFile)

		// Start the ID at 1 because gorm thinks it's a blank field if 0 is
		// passed and will automatically derive a value for it.
		invoiceFile.ID = int64(idx + 1)

		invoiceFile.InvoiceToken = token
		invoiceFile.InvoiceVersion = version

		err := c.db.Create(invoiceFile).Error
		if err != nil {
			return err
		}
	}

	return nil
}

// Deletes all data from all tables.
//
// DeleteAllData satisfies the backend interface.
func (c *cockroachdb) DeleteAllData() error {
	log.Debugf("DeleteAllData")

	c.dropTable(tableNameInvoicePayment)
	c.dropTable(tableNameInvoiceChange)
	c.dropTable(tableNameInvoice)
	c.dropTable(tableNameIdentity)
	c.dropTable(tableNameUser)
	return nil
}

// Close shuts down the database.
//
// Close satisfies the backend interface.
func (c *cockroachdb) Close() error {
	return c.db.Close()
}

// New creates a new cockroachdb instance.
func New(dataDir, dbName, username, host string) (*cockroachdb, error) {
	log.Tracef("cockroachdb New")

	cockroachDBFile := filepath.Join(dataDir, "cockroachdb")

	sslCA := filepath.Join(cockroachDBFile, "ca.crt")
	sslCert := filepath.Join(cockroachDBFile, fmt.Sprintf("client.%v.crt", username))
	sslKey := filepath.Join(cockroachDBFile, fmt.Sprintf("client.%v.key", username))

	// verify CA certificate exists and is readable.
	f, err := os.Open(sslCA)
	if err != nil {
		return nil, err
	}
	_ = f.Close()

	// verify client keypair
	if _, err := tls.LoadX509KeyPair(sslCert, sslKey); err != nil {
		return nil, err
	}

	v := url.Values{}
	v.Set("sslrootcert", sslCA)
	v.Set("sslcert", sslCert)
	v.Set("sslkey", sslKey)
	v.Set("sslmode", "verify-full")
	dataSource := fmt.Sprintf("postgresql://%v@%v/%v?%v", username, host,
		dbName, v.Encode())

	db, err := gorm.Open("postgres", dataSource)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	db.LogMode(true)

	c := cockroachdb{
		db: db,
	}

	err = c.dropTable(tableNameInvoiceChange)
	if err != nil {
		return nil, fmt.Errorf("error dropping %v table: %v",
			tableNameInvoiceChange, err)
	}
	err = c.dropTable(tableNameInvoicePayment)
	if err != nil {
		return nil, fmt.Errorf("error dropping %v table: %v",
			tableNameInvoicePayment, err)
	}
	err = c.dropTable(tableNameInvoiceFile)
	if err != nil {
		return nil, fmt.Errorf("error dropping %v table: %v",
			tableNameInvoiceFile, err)
	}
	err = c.dropTable(tableNameInvoice)
	if err != nil {
		return nil, fmt.Errorf("error dropping %v table: %v", tableNameInvoice,
			err)
	}

	c.db.AutoMigrate(
		&User{},
		&Identity{},
		&Invoice{},
		&InvoiceFile{},
		&InvoiceChange{},
		&InvoicePayment{},
	)

	return &c, nil
}
