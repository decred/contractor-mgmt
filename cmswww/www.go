package main

import (
	"crypto/elliptic"
	"crypto/tls"
	_ "encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/politeia/util"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
	"github.com/decred/contractor-mgmt/cmswww/database/cockroachdb"
)

type permission uint

const (
	permissionPublic permission = iota
	permissionLogin
	permissionAdmin

	csrfKeyLength = 32

	// indexFile contains the file name of the index file
	indexFile = "index.md"

	// mdStream* indicate the metadata stream used for various types
	mdStreamGeneral  = 0 // General information for this invoice
	mdStreamChanges  = 1 // Changes to the invoice status
	mdStreamPayments = 2 // Payments made for this invoice

	VersionBackendInvoiceMetadata  = 1
	VersionBackendInvoiceMDChange  = 1
	VersionBackendInvoiceMDPayment = 1
)

// cmswww application context.
type cmswww struct {
	sync.RWMutex // lock for inventory and caches

	cfg    *config
	router *mux.Router

	store *sessions.FilesystemStore

	db             database.Database
	params         *chaincfg.Params
	client         *http.Client // politeiad client
	eventManager   *EventManager
	polledPayments map[string]polledPayment // [token][polledPayment]

	// Following entries require locks
	inventoryLoaded bool // Current inventory
}

// RespondWithError returns an HTTP error status to the client. If it's a user
// error, it returns a 4xx HTTP status and the specific user error code. If it's
// an internal server error, it returns 500 and an error code which is also
// outputted to the logs so that it can be correlated later if the user
// files a complaint.
func RespondWithError(
	w http.ResponseWriter,
	r *http.Request,
	userHTTPCode int,
	format string,
	args ...interface{},
) {
	if userErr, ok := args[0].(v1.UserError); ok {
		if userHTTPCode == 0 {
			userHTTPCode = http.StatusBadRequest
		}

		if len(userErr.ErrorContext) == 0 {
			log.Errorf("RespondWithError: %v %v",
				int64(userErr.ErrorCode),
				v1.ErrorStatus[userErr.ErrorCode])
		} else {
			log.Errorf("RespondWithError: %v %v: %v",
				int64(userErr.ErrorCode),
				v1.ErrorStatus[userErr.ErrorCode],
				strings.Join(userErr.ErrorContext, ", "))
		}

		util.RespondWithJSON(w, userHTTPCode,
			v1.ErrorReply{
				ErrorCode:    int64(userErr.ErrorCode),
				ErrorContext: userErr.ErrorContext,
			})
		return
	}

	if pdError, ok := args[0].(v1.PDError); ok {
		pdErrorCode := convertErrorStatusFromPD(pdError.ErrorReply.ErrorCode)
		if pdErrorCode == v1.ErrorStatusInvalid {
			errorCode := time.Now().Unix()
			log.Errorf("%v %v %v %v Internal error %v: error "+
				"code from politeiad: %v", remoteAddr(r),
				r.Method, r.URL, r.Proto, errorCode,
				pdError.ErrorReply.ErrorCode)
			util.RespondWithJSON(w, http.StatusInternalServerError,
				v1.ErrorReply{
					ErrorCode: errorCode,
				})
			return
		}

		util.RespondWithJSON(w, pdError.HTTPCode,
			v1.ErrorReply{
				ErrorCode:    int64(pdErrorCode),
				ErrorContext: pdError.ErrorReply.ErrorContext,
			})
		return
	}

	errorCode := time.Now().Unix()
	ec := fmt.Sprintf("%v %v %v %v Internal error %v: ", remoteAddr(r),
		r.Method, r.URL, r.Proto, errorCode)
	log.Errorf(ec+format, args...)
	log.Errorf("Stacktrace: %s", debug.Stack())
	util.RespondWithJSON(w, http.StatusInternalServerError,
		v1.ErrorReply{
			ErrorCode: errorCode,
		})
}

// version is an HTTP GET to determine what version and API route this backend
// is using.  Additionally it is used to obtain a CSRF token.
func (c *cmswww) HandleVersion(w http.ResponseWriter, r *http.Request) {
	log.Tracef("HandleVersion")

	vr := v1.VersionReply{
		Version:   v1.APIVersion,
		Route:     v1.APIRoute,
		PublicKey: hex.EncodeToString(c.cfg.Identity.Key[:]),
		TestNet:   c.cfg.TestNet,
	}

	// Check if there's an invalid session that the client thinks is active.
	replacedSession, err := c.ReplaceSessionIfInvalid(w, r)
	if err != nil {
		RespondWithError(w, r, 0, "HandleVersion: session.Save %v", err)
		return
	}

	// Fetch the session user, if present, and add it to the reply.
	var user *database.User
	if !replacedSession {
		user, err = c.GetSessionUser(r)
		if err != nil && err != database.ErrUserNotFound {
			RespondWithError(w, r, 0,
				"HandleVersion: GetSessionUser %v", err)
			return
		}
	}

	if user != nil {
		vr.User, err = c.CreateLoginReply(user, user.LastLogin)
		if err != nil {
			RespondWithError(w, r, 0,
				"HandleVersion: CreateLoginReply %v", err)
			return
		}
	}

	reply, err := json.Marshal(vr)
	if err != nil {
		RespondWithError(w, r, 0, "HandleVersion: Marshal %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Strict-Transport-Security",
		"max-age=63072000; includeSubDomains")
	w.Header().Set(v1.CsrfToken, csrf.Token(r))
	w.WriteHeader(http.StatusOK)
	w.Write(reply)
}

// handlePolicy returns details on how to interact with the server.
func (c *cmswww) HandlePolicy(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	return &v1.PolicyReply{
		MinPasswordLength:      v1.PolicyMinPasswordLength,
		MinUsernameLength:      v1.PolicyMinUsernameLength,
		MaxUsernameLength:      v1.PolicyMaxUsernameLength,
		UsernameSupportedChars: v1.PolicyUsernameSupportedChars,
		ListPageSize:           v1.ListPageSize,
		ValidMIMETypes: []string{
			"text/plain; charset=utf-8",
		},
		Invoice: v1.InvoicePolicy{
			FieldDelimiterChar: v1.PolicyInvoiceFieldDelimiterChar,
			CommentChar:        v1.PolicyInvoiceCommentChar,
			Fields:             v1.InvoiceFields,
		},
	}, nil
}

// HandleNotFound is a generic handler for an invalid route.
func (c *cmswww) HandleNotFound(w http.ResponseWriter, r *http.Request) {
	// Log incoming connection
	log.Debugf("Invalid route: %v %v %v %v", remoteAddr(r), r.Method, r.URL,
		r.Proto)

	// Trace incoming request
	log.Tracef("%v", newLogClosure(func() string {
		trace, err := httputil.DumpRequest(r, true)
		if err != nil {
			trace = []byte(fmt.Sprintf("logging: "+
				"DumpRequest %v", err))
		}
		return string(trace)
	}))

	util.RespondWithJSON(w, http.StatusNotFound, v1.ErrorReply{})
}

func _main() error {
	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	loadedCfg, _, err := loadConfig()
	if err != nil {
		return fmt.Errorf("Could not load configuration file: %v", err)
	}
	defer func() {
		if logRotator != nil {
			logRotator.Close()
		}
	}()

	log.Infof("Version : %v", version())
	log.Infof("Network : %v", activeNetParams.Params.Name)
	log.Infof("Home dir: %v", loadedCfg.HomeDir)

	// Create the data directory in case it does not exist.
	err = os.MkdirAll(loadedCfg.DataDir, 0700)
	if err != nil {
		return err
	}

	// Generate the TLS cert and key file if both don't already
	// exist.
	if !fileExists(loadedCfg.HTTPSKey) &&
		!fileExists(loadedCfg.HTTPSCert) {
		log.Infof("Generating HTTPS keypair...")

		err := util.GenCertPair(elliptic.P256(), "cmswww",
			loadedCfg.HTTPSCert, loadedCfg.HTTPSKey)
		if err != nil {
			return fmt.Errorf("unable to create https keypair: %v",
				err)
		}

		log.Infof("HTTPS keypair created...")
	}

	// Setup application context.
	c := &cmswww{
		cfg:            loadedCfg,
		params:         activeNetParams.Params,
		polledPayments: make(map[string]polledPayment),
	}

	// Check if this command is being run to fetch the identity.
	if c.cfg.FetchIdentity {
		return c.RemoteIdentity()
	}

	// Setup database.
	cockroachdb.UseLogger(cockroachdbLog)
	c.db, err = cockroachdb.New(c.cfg.DataDir, c.cfg.CockroachDBName,
		c.cfg.CockroachDBUsername, c.cfg.CockroachDBHost)
	if err != nil {
		return err
	}

	// Setup events
	c.initEventManager()

	// Try to load inventory but do not fail.
	log.Infof("Attempting to load invoice inventory")
	err = c.LoadInventory()
	if err != nil {
		log.Errorf("LoadInventory: %v", err)
	}

	// Load or create new CSRF key
	log.Infof("Load CSRF key")
	csrfKeyFilename := filepath.Join(c.cfg.DataDir, "csrf.key")
	fCSRF, err := os.Open(csrfKeyFilename)
	if err != nil {
		if os.IsNotExist(err) {
			key, err := util.Random(csrfKeyLength)
			if err != nil {
				return err
			}

			// Persist key
			fCSRF, err = os.OpenFile(csrfKeyFilename,
				os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				return err
			}
			_, err = fCSRF.Write(key)
			if err != nil {
				return err
			}
			_, err = fCSRF.Seek(0, 0)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	csrfKey := make([]byte, csrfKeyLength)
	r, err := fCSRF.Read(csrfKey)
	if err != nil {
		return err
	}
	if r != csrfKeyLength {
		return fmt.Errorf("CSRF key corrupt")
	}
	fCSRF.Close()

	// Set up the code that checks for invoice payments.
	err = c.initPaymentChecker()
	if err != nil {
		return err
	}

	// Make sure the cookie path is explicitly set to the root path, to fix
	// an issue where multiple CSRF tokens were being stored in the cookie.
	csrfHandle := csrf.Protect(csrfKey, csrf.Path("/"))

	c.SetupRoutes()
	err = c.SetupSessions()
	if err != nil {
		return err
	}

	// Bind to a port and pass our router in
	listenC := make(chan error)
	for _, listener := range loadedCfg.Listeners {
		listen := listener
		go func() {
			cfg := &tls.Config{
				MinVersion: tls.VersionTLS12,
				CurvePreferences: []tls.CurveID{
					tls.CurveP256, // BLAME CHROME, NOT ME!
					tls.CurveP521,
					tls.X25519},
				PreferServerCipherSuites: true,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				},
			}
			srv := &http.Server{
				Addr:      listen,
				TLSConfig: cfg,
				TLSNextProto: make(map[string]func(*http.Server,
					*tls.Conn, http.Handler)),
			}
			srv.Handler = csrfHandle(c.router)
			log.Infof("Listen: %v", listen)
			listenC <- srv.ListenAndServeTLS(loadedCfg.HTTPSCert,
				loadedCfg.HTTPSKey)
		}()
	}

	// Tell user we are ready to go.
	log.Infof("Start of day")

	// Setup OS signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGINT)
	for {
		select {
		case sig := <-sigs:
			log.Infof("Terminating with %v", sig)
			goto done
		case err := <-listenC:
			log.Errorf("%v", err)
			goto done
		}
	}
done:
	log.Infof("Exiting")
	return nil
}

func main() {
	err := _main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
