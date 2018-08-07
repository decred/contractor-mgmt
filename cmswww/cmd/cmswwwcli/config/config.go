package config

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/politeia/politeiad/api/v1/identity"
	flags "github.com/jessevdk/go-flags"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/sharedconfig"
)

const (
	defaultHost       = "https://127.0.0.1:4443"
	defaultWalletHost = "https://127.0.0.1" // Only allow localhost for now
	FaucetURL         = "https://faucet.decred.org/requestfaucet"

	defaultWalletMainnetPort = "19110"
	defaultWalletTestnetPort = "19111"

	ErrorNoUserIdentity   = "No user identity found."
	ErrorBeforeAfterFlags = "The 'before' and 'after' flags cannot be used at " +
		"the same time."
)

var (
	dcrwalletHomeDir      = dcrutil.AppDataDir("dcrwallet", false)
	defaultHomeDir        = filepath.Join(sharedconfig.DefaultHomeDir, "cli")
	defaultWalletCertFile = filepath.Join(dcrwalletHomeDir, "rpc.cert")

	HomeDir    = defaultHomeDir
	WalletCert = defaultWalletCertFile
	// only allow testnet wallet host for now
	WalletHost = defaultWalletHost + ":" + defaultWalletTestnetPort

	// cli params
	Host    = defaultHost
	JSONOut bool
	Verbose bool

	Cookies              []*http.Cookie
	CsrfToken            string
	LoggedInUser         *v1.LoginReply
	LoggedInUserIdentity *identity.FullIdentity

	cookieFile string
	csrfFile   string
)

// cli flags
type config struct {
	Host    func(string) error `long:"host" description:"cmswww host"`
	JSONOut bool               `long:"jsonout" description:"Output only the last command's JSON output; use this option when writing scripts"`
	Verbose bool               `short:"v" long:"verbose" description:"Print request and response details"`
}

func stringToHash(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprint(h.Sum32())
}

func getUserIdentityFile(email string) string {
	userIdentityFilename := "identity_" + email + "_" + stringToHash(Host) + ".json"
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

// create a user identity filename that is unique for each host.  This makes
// it possible to interact with multiple hosts simultaneously.
func setCookieFile(host string) {
	cookieFilename := "cookie_" + stringToHash(host) + ".json"
	cookieFile = filepath.Join(HomeDir, cookieFilename)
}

// create a csrf token filename that is unique for each host.   This makes
// it possible to interact with multiple hosts simultaneously.
func setCsrfFile(host string) {
	csrfFilename := "csrf_" + stringToHash(host) + ".json"
	csrfFile = filepath.Join(HomeDir, csrfFilename)
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
	err := os.MkdirAll(HomeDir, 0700)
	if err != nil {
		return err
	}

	// Default Cfg.
	cfg := &config{
		JSONOut: false,
		Verbose: false,
	}

	cfg.Host = func(host string) error {
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			return fmt.Errorf("host must begin with http:// or https://")
		}
		Host = host
		return nil
	}

	parser := flags.NewParser(cfg, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	JSONOut = cfg.JSONOut
	Verbose = cfg.Verbose

	// load user identity
	/*
		setUserIdentityFile(Host)
		if fileExists(UserIdentityFile) {
			UserIdentity, err = loadUserIdentity(UserIdentityFile)
			if err != nil {
				return nil, err
			}
		}
	*/
	// load cookies
	setCookieFile(Host)
	if fileExists(cookieFile) {
		Cookies, err = LoadCookies()
		if err != nil {
			return err
		}
	}

	// load CSRF token
	setCsrfFile(Host)
	if fileExists(csrfFile) {
		CsrfToken, err = loadCsrf()
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteCookies() error {
	return os.Remove(cookieFile)
}

func SaveCookies(cookies []*http.Cookie) error {
	ck, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("could not marshal cookies")
	}

	err = ioutil.WriteFile(cookieFile, ck, 0600)
	if err != nil {
		return err
	}
	/*
		if Verbose {
			fmt.Printf("Cookies saved to: %v\n", cookieFile)
		}
	*/
	return nil
}

func LoadCookies() ([]*http.Cookie, error) {
	b, err := ioutil.ReadFile(cookieFile)
	if err != nil {
		return nil, err
	}

	ck := []*http.Cookie{}
	err = json.Unmarshal(b, &ck)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal cookies")
	}

	return ck, nil
}

func SaveCsrf(csrf string) error {
	err := ioutil.WriteFile(csrfFile, []byte(csrf), 0600)
	if err != nil {
		return err
	}
	/*
		if Verbose {
			fmt.Printf("CSRF token saved to: %v\n", csrfFile)
		}
	*/
	return nil
}

func loadCsrf() (string, error) {
	b, err := ioutil.ReadFile(csrfFile)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
