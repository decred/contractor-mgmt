package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/decred/politeia/politeiad/api/v1/identity"
	"github.com/kennygrant/sanitize"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/sharedconfig"
)

const (
	defaultHost = "https://127.0.0.1:4443"
	FaucetURL   = "https://faucet.decred.org/requestfaucet"

	ErrorNoUserIdentity   = "No user identity found."
	ErrorBeforeAfterFlags = "The 'before' and 'after' flags cannot be used at " +
		"the same time."
)

var (
	HomeDir     = filepath.Join(sharedconfig.DefaultHomeDir, "cli")
	InvoicesDir = filepath.Join(HomeDir, "invoices")

	// cli params
	Host       = defaultHost
	JSONOutput bool
	Verbose    bool

	SuppressOutput bool

	Cookies              []*http.Cookie
	CsrfToken            string
	LoggedInUser         *v1.LoginReply
	LoggedInUserIdentity *identity.FullIdentity

	ServerPublicKey string
)

func getUserIdentityFile(email string) string {
	userIdentityFilename := "id_" + sanitize.BaseName(email) + "_" + sanitize.BaseName(Host) + ".json"
	return filepath.Join(HomeDir, userIdentityFilename)
}

func SaveUserIdentity(id *identity.FullIdentity, email string) error {
	return id.Save(getUserIdentityFile(email))
}

func DeleteUserIdentity(email string) error {
	return os.Remove(getUserIdentityFile(email))
}

func LoadUserIdentity(email string) (*identity.FullIdentity, error) {
	id, err := identity.LoadFullIdentity(getUserIdentityFile(email))
	LoggedInUserIdentity = id
	return id, err
}

func GetInvoiceMonthStr(month, year uint16) string {
	t := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return t.Format("2006-01")
}

func GetInvoiceDirectory() string {
	return filepath.Join(InvoicesDir, LoggedInUser.Email)
}

func GetInvoiceFilename(month, year uint16) (string, error) {
	filename := filepath.Join(GetInvoiceDirectory(),
		fmt.Sprintf("%v.csv", GetInvoiceMonthStr(month, year)))

	if !FileExists(filename) {
		dirpath := filepath.Dir(filename)
		if err := os.MkdirAll(dirpath, 0700); err != nil {
			return "", err
		}
	}

	return filename, nil
}

func GetInvoiceSubmissionRecordFilename(month, year uint16) (string, error) {
	filename := filepath.Join(GetInvoiceDirectory(),
		fmt.Sprintf("submission_record_%v.json", GetInvoiceMonthStr(month, year)))

	if !FileExists(filename) {
		dirpath := filepath.Dir(filename)
		if err := os.MkdirAll(dirpath, 0700); err != nil {
			return "", err
		}
	}

	return filename, nil
}

func GetMonthAndYearFromInvoice(filename string) (uint16, uint16, error) {
	if !strings.HasSuffix(filename, ".csv") {
		return 0, 0, fmt.Errorf("not an invoice file")
	}

	pair := strings.Split(filename[:len(filename)-4], "-")
	year, err := strconv.ParseUint(pair[0], 10, 16)
	if err != nil {
		return 0, 0, err
	}

	month, err := strconv.ParseUint(pair[1], 10, 16)
	return uint16(year), uint16(month), err
}

// Returns a cookie filename that is unique for each host.  This makes
// it possible to interact with multiple hosts simultaneously.
func getCookieFile() string {
	cookieFilename := "cookie_" + sanitize.BaseName(Host) + ".json"
	return filepath.Join(HomeDir, cookieFilename)
}

// Returns a csrf token filename that is unique for each host.   This makes
// it possible to interact with multiple hosts simultaneously.
func getCsrfFile() string {
	csrfFilename := "csrf_" + sanitize.BaseName(Host) + ".json"
	return filepath.Join(HomeDir, csrfFilename)
}

// FileExists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func Load() error {
	// create home and invoices directories if they don't already exist
	if err := os.MkdirAll(InvoicesDir, 0700); err != nil {
		return err
	}

	if err := LoadCsrf(); err != nil {
		return err
	}

	return LoadCookies()
}

func DeleteCookies() error {
	return os.Remove(getCookieFile())
}

func SaveCookies(cookies []*http.Cookie) error {
	Cookies = cookies
	ck, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("could not marshal cookies")
	}

	return ioutil.WriteFile(getCookieFile(), ck, 0600)
}

func LoadCookies() error {
	if !FileExists(getCookieFile()) {
		return nil
	}

	b, err := ioutil.ReadFile(getCookieFile())
	if err != nil {
		return err
	}

	ck := []*http.Cookie{}
	err = json.Unmarshal(b, &ck)
	if err != nil {
		return fmt.Errorf("could not unmarshal cookies")
	}

	Cookies = ck
	return nil
}

func SaveCsrf(csrf string) error {
	CsrfToken = csrf
	return ioutil.WriteFile(getCsrfFile(), []byte(csrf), 0600)
}

func LoadCsrf() error {
	if !FileExists(getCsrfFile()) {
		return nil
	}

	b, err := ioutil.ReadFile(getCsrfFile())
	if err != nil {
		return err
	}

	CsrfToken = string(b)
	return nil
}
