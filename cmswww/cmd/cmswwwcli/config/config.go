package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

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
	defaultHomeDir = filepath.Join(sharedconfig.DefaultHomeDir, "cli")

	HomeDir = defaultHomeDir

	// cli params
	Host       = defaultHost
	JSONOutput bool
	Verbose    bool

	SuppressOutput bool

	Cookies              []*http.Cookie
	CsrfToken            string
	LoggedInUser         *v1.LoginReply
	LoggedInUserIdentity *identity.FullIdentity
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

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func Load() error {
	// create home directory if it doesn't already exist
	if err := os.MkdirAll(HomeDir, 0700); err != nil {
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
	if !fileExists(getCookieFile()) {
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
	if !fileExists(getCsrfFile()) {
		return nil
	}

	b, err := ioutil.ReadFile(getCsrfFile())
	if err != nil {
		return err
	}

	CsrfToken = string(b)
	return nil
}
