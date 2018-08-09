package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	cliconfig "github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
	wwwconfig "github.com/decred/contractor-mgmt/cmswww/sharedconfig"
	"github.com/decred/dcrd/dcrutil"
)

type beforeVerifyReply func() interface{}
type verifyReply func() bool

const (
	cli    = "cmswwwcli"
	dbutil = "cmswwwdbutil"
)

var (
	cfg          *config
	politeiadCmd *exec.Cmd
	cmswwwCmd    *exec.Cmd
)

func executeCommand(args ...string) *exec.Cmd {
	if cfg.Verbose {
		fmt.Printf("  $ %v\n", strings.Join(args, " "))
	}
	return exec.Command(args[0], args[1:]...)
}

func createCmswwwCmd() *exec.Cmd {
	return executeCommand(
		"cmswww",
		"--testnet",
		"--mailhost", "",
		"--mailuser", "",
		"--mailpass", "",
		"--webserveraddress", "",
		"--debuglevel", cfg.DebugLevel)
}

func createLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
}

func waitForStartOfDay(out io.Reader) {
	buf := bufio.NewScanner(out)
	for buf.Scan() {
		text := buf.Text()
		if strings.Contains(text, "Start of day") {
			return
		}
	}
}

func startCmswww() error {
	fmt.Printf("Starting cmswww\n")
	cmswwwCmd = createCmswwwCmd()
	out, _ := cmswwwCmd.StdoutPipe()
	if err := cmswwwCmd.Start(); err != nil {
		cmswwwCmd = nil
		return err
	}

	logFile, err := createLogFile(cfg.CmswwwLogFile)
	if err != nil {
		return err
	}

	reader := io.TeeReader(out, logFile)
	waitForStartOfDay(reader)
	go io.Copy(logFile, out)

	// Get the version for the csrf
	return getVersionFromCmswww()
}

func startPoliteiad() error {
	fmt.Printf("Starting politeiad\n")
	politeiadCmd = executeCommand("politeiad", "--testnet")
	out, _ := politeiadCmd.StdoutPipe()
	if err := politeiadCmd.Start(); err != nil {
		politeiadCmd = nil
		return err
	}

	logFile, err := createLogFile(cfg.PoliteiadLogFile)
	if err != nil {
		return err
	}

	reader := io.TeeReader(out, logFile)
	waitForStartOfDay(reader)
	return nil
}

func getVersionFromCmswww() error {
	fmt.Printf("Getting version\n")

	var vr *v1.VersionReply
	return executeCliCommand(
		func() interface{} {
			vr = &v1.VersionReply{}
			return vr
		},
		func() bool {
			return vr.PublicKey != ""
		},
		"version",
	)
}

func inviteUser(email string) (string, error) {
	fmt.Printf("Inviting user: %v\n", email)

	var inur *v1.InviteNewUserReply
	err := executeCliCommand(
		func() interface{} {
			inur = &v1.InviteNewUserReply{}
			return inur
		},
		func() bool {
			return inur.VerificationToken != ""
		},
		"invite",
		email,
	)

	return inur.VerificationToken, err
}

func registerUser(email, username, password, token string) error {
	fmt.Printf("Registering user: %v\n", email)

	var rr *v1.RegisterReply
	return executeCliCommand(
		func() interface{} {
			rr = &v1.RegisterReply{}
			return rr
		},
		func() bool {
			return true
		},
		"register",
		email,
		username,
		password,
		token,
	)
}

func createAdminUserWithDBUtil(email, username, password string) error {
	fmt.Printf("Creating admin user: %v\n", cfg.AdminEmail)
	cmd := executeCommand(
		dbutil,
		"-testnet",
		"-createadmin",
		email,
		username,
		password,
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func setNewIdentity(email, password string) error {
	if err := login(cfg.AdminEmail, cfg.AdminPass); err != nil {
		return err
	}

	token, err := newIdentity()
	if err != nil {
		return err
	}

	if err = verifyIdentity(token); err != nil {
		return err
	}

	return logout()
}

func createContractorUser(
	adminEmail,
	adminPass,
	contractorEmail,
	contractorUser,
	contractorPass string,
) error {
	if err := login(adminEmail, adminPass); err != nil {
		return err
	}

	token, err := inviteUser(contractorEmail)
	if err != nil {
		return err
	}

	if err = logout(); err != nil {
		return err
	}

	return registerUser(contractorEmail, contractorUser, contractorPass, token)
}

func me() (*v1.LoginReply, error) {
	fmt.Printf("Fetching user details\n")
	var lr *v1.LoginReply
	err := executeCliCommand(
		func() interface{} {
			lr = &v1.LoginReply{}
			return lr
		},
		func() bool {
			return lr.UserID != ""
		},
		"me")
	if err != nil {
		return nil, err
	}
	return lr, nil
}

func executeCliCommand(beforeVerify beforeVerifyReply, verify verifyReply, args ...string) error {
	fullArgs := make([]string, 0, len(args)+2)
	fullArgs = append(fullArgs, cli)
	fullArgs = append(fullArgs, "--host")
	fullArgs = append(fullArgs, "https://127.0.0.1:4443")
	fullArgs = append(fullArgs, "--jsonout")
	fullArgs = append(fullArgs, args...)
	cmd := executeCommand(fullArgs...)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()

	errBytes, err := ioutil.ReadAll(stderr)
	if err != nil {
		return err
	}

	if len(errBytes) > 0 {
		return fmt.Errorf("unexpected error output from %v: %v", cli,
			string(errBytes))
	}

	var allText string
	buf := bufio.NewScanner(stdout)
	for buf.Scan() {
		text := buf.Text()

		var lines []string
		if strings.Contains(text, "\n") {
			lines = strings.Split(text, "\n")
		} else {
			lines = append(lines, text)
		}

		for _, line := range lines {
			if cfg.Verbose {
				fmt.Printf("  %v\n", line)
			}

			var er v1.ErrorReply
			err := json.Unmarshal([]byte(line), &er)
			if err == nil && er.ErrorCode != int64(v1.ErrorStatusInvalid) {
				return fmt.Errorf("error returned from %v: %v %v", cli,
					er.ErrorCode, er.ErrorContext)
			}

			reply := beforeVerify()
			err = json.Unmarshal([]byte(line), reply)
			if err == nil && verify() {
				return nil
			}

			allText += line + "\n"
		}
	}

	if err := buf.Err(); err != nil {
		return err
	}

	return fmt.Errorf("unexpected output from %v: %v", cli, allText)
}

func login(email, password string) error {
	fmt.Printf("Logging in as: %v\n", email)
	var lr *v1.LoginReply
	return executeCliCommand(
		func() interface{} {
			lr = &v1.LoginReply{}
			return lr
		},
		func() bool {
			return lr.UserID != ""
		},
		"login",
		email,
		password)
}

func logout() error {
	cmd := executeCommand(cli, "logout")
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func newIdentity() (string, error) {
	fmt.Printf("Generating new identity\n")
	var nir *v1.NewIdentityReply
	err := executeCliCommand(
		func() interface{} {
			nir = &v1.NewIdentityReply{}
			return nir
		},
		func() bool {
			return nir.VerificationToken != ""
		},
		"newidentity",
	)

	return nir.VerificationToken, err
}

func verifyIdentity(token string) error {
	fmt.Printf("Verifying new identity\n")
	var vnir *v1.VerifyNewIdentityReply
	return executeCliCommand(
		func() interface{} {
			vnir = &v1.VerifyNewIdentityReply{}
			return vnir
		},
		func() bool {
			return true
		},
		"verifyidentity",
		token,
	)
}

func handleError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func deleteExistingData() error {
	fmt.Printf("Deleting existing data\n")

	// politeiad data dir
	politeiadDataDir := filepath.Join(dcrutil.AppDataDir("politeiad", false), "data")
	if err := os.RemoveAll(politeiadDataDir); err != nil {
		return err
	}

	// cmswww data dir
	if err := os.RemoveAll(wwwconfig.DefaultDataDir); err != nil {
		return err
	}

	// cmswww cli dir
	return os.RemoveAll(cliconfig.HomeDir)
}

func stopPoliteiad() {
	if politeiadCmd != nil {
		fmt.Printf("Stopping politeiad\n")
		politeiadCmd.Process.Kill()
		politeiadCmd = nil
	}
}

func stopCmswww() {
	if cmswwwCmd != nil {
		fmt.Printf("Stopping cmswww\n")
		cmswwwCmd.Process.Kill()
		cmswwwCmd = nil
	}
}

func stopServers() {
	stopPoliteiad()
	stopCmswww()
}

func _main() error {
	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	var err error
	cfg, err = loadConfig()
	if err != nil {
		return fmt.Errorf("Could not load configuration file: %v", err)
	}

	if cfg.DeleteData {
		if err = deleteExistingData(); err != nil {
			return err
		}
	}

	err = createAdminUserWithDBUtil(cfg.AdminEmail, cfg.AdminUser,
		cfg.AdminPass)
	if err != nil {
		return err
	}

	if err = startPoliteiad(); err != nil {
		return err
	}

	if err = startCmswww(); err != nil {
		return err
	}

	if err = setNewIdentity(cfg.AdminEmail, cfg.AdminPass); err != nil {
		return err
	}

	err = createContractorUser(
		cfg.AdminEmail,
		cfg.AdminPass,
		cfg.ContractorEmail,
		cfg.ContractorUser,
		cfg.ContractorPass,
	)
	if err != nil {
		return err
	}

	if err = setNewIdentity(cfg.ContractorEmail, cfg.ContractorPass); err != nil {
		return err
	}

	fmt.Printf("Load data complete\n")
	return nil
}

func main() {
	err := _main()
	stopServers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
