package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cliconfig "github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
	wwwconfig "github.com/decred/contractor-mgmt/cmswww/sharedconfig"
	"github.com/decred/dcrd/dcrutil"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwdataload/client"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwdataload/config"
)

var (
	cfg          *config.Config
	c            *client.Client
	politeiadCmd *exec.Cmd
	cmswwwCmd    *exec.Cmd
)

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
	cmswwwCmd = c.CreateCmswwwCmd()
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
	return c.Version()
}

func startPoliteiad() error {
	fmt.Printf("Starting politeiad\n")
	politeiadCmd = c.ExecuteCommand("politeiad", "--testnet")
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

func setNewIdentity(email, password string) error {
	if err := c.Login(email, password); err != nil {
		return err
	}

	token, err := c.NewIdentity()
	if err != nil {
		return err
	}

	if err = c.VerifyIdentity(token); err != nil {
		return err
	}

	return c.Logout()
}

func createContractorUser(
	adminEmail,
	adminPass,
	contractorEmail,
	contractorUser,
	contractorPass string,
) error {
	if err := c.Login(adminEmail, adminPass); err != nil {
		return err
	}

	_, err := c.InviteUser(contractorEmail)
	if err != nil {
		return err
	}

	token, err := c.ResendInvite(contractorEmail)
	if err != nil {
		return err
	}

	if err = c.Logout(); err != nil {
		return err
	}

	return c.RegisterUser(contractorEmail, contractorUser, contractorPass, token)
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
	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("Could not load configuration file: %v", err)
	}

	c = client.NewClient(cfg)

	if cfg.DeleteData {
		if err = deleteExistingData(); err != nil {
			return err
		}
	}

	err = c.CreateAdminUser(cfg.AdminEmail, cfg.AdminUser, cfg.AdminPass)
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
