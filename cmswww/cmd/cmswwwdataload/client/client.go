package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwdataload/config"
)

type Client struct {
	cfg *config.Config
}

type beforeVerifyReply func() interface{}
type verifyReply func() bool

const (
	cli    = "cmswwwcli"
	dbutil = "cmswwwdbutil"
)

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
	}
}

func (c *Client) ExecuteCommand(args ...string) *exec.Cmd {
	if c.cfg.Verbose {
		fmt.Printf("  $ %v\n", strings.Join(args, " "))
	}
	return exec.Command(args[0], args[1:]...)
}

func (c *Client) PerformErrorHandling(cmd *exec.Cmd, stderr io.ReadCloser) error {
	errBytes, err := ioutil.ReadAll(stderr)
	if err != nil {
		return err
	}

	if len(errBytes) > 0 {
		return fmt.Errorf("unexpected error output: %v",
			strings.TrimSpace(string(errBytes)))
	}

	return nil
}

func (c *Client) ExecuteCommandWithErrorHandling(args ...string) (*exec.Cmd, error) {
	cmd := c.ExecuteCommand(args...)

	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer cmd.Wait()

	err := c.PerformErrorHandling(cmd, stderr)
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func (c *Client) ExecuteCommandAndWait(args ...string) error {
	cmd := c.ExecuteCommand(args...)

	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}

	err := c.PerformErrorHandling(cmd, stderr)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

func (c *Client) CreateCmswwwCmd() *exec.Cmd {
	return c.ExecuteCommand(
		"cmswww",
		"--testnet",
		"--mailhost", "",
		"--mailuser", "",
		"--mailpass", "",
		"--webserveraddress", "",
		"--debuglevel", c.cfg.DebugLevel)
}

func (c *Client) ExecuteCliCommand(reply interface{}, verify verifyReply, args ...string) error {
	fullArgs := make([]string, 0, len(args)+2)
	fullArgs = append(fullArgs, cli)
	fullArgs = append(fullArgs, "--host")
	fullArgs = append(fullArgs, "https://127.0.0.1:4443")
	fullArgs = append(fullArgs, "--jsonout")
	fullArgs = append(fullArgs, args...)
	cmd := c.ExecuteCommand(fullArgs...)

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
			if c.cfg.Verbose {
				fmt.Printf("  %v\n", line)
			}

			var er v1.ErrorReply
			err := json.Unmarshal([]byte(line), &er)
			if err == nil && er.ErrorCode != int64(v1.ErrorStatusInvalid) {
				return fmt.Errorf("error returned from %v: %v %v", cli,
					er.ErrorCode, er.ErrorContext)
			}

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

func (c *Client) Version() error {
	fmt.Printf("Getting version\n")

	var vr v1.VersionReply
	return c.ExecuteCliCommand(
		&vr,
		func() bool {
			return vr.PublicKey != ""
		},
		"version",
	)
}

func (c *Client) Login(email, password string) error {
	fmt.Printf("Logging in as: %v\n", email)
	var lr v1.LoginReply
	return c.ExecuteCliCommand(
		&lr,
		func() bool {
			return lr.UserID != ""
		},
		"login",
		email,
		password)
}

func (c *Client) Logout() error {
	fmt.Printf("Logging out\n")
	return c.ExecuteCommandAndWait(cli, "logout")
}

func (c *Client) NewIdentity() (string, error) {
	fmt.Printf("Generating new identity\n")
	var nir v1.NewIdentityReply
	err := c.ExecuteCliCommand(
		&nir,
		func() bool {
			return nir.VerificationToken != ""
		},
		"newidentity",
	)

	return nir.VerificationToken, err
}

func (c *Client) VerifyIdentity(token string) error {
	fmt.Printf("Verifying new identity\n")
	return c.ExecuteCommandAndWait(cli, "verifyidentity", token)
}

func (c *Client) InviteUser(email string) (string, error) {
	fmt.Printf("Inviting user: %v\n", email)

	var inur v1.InviteNewUserReply
	err := c.ExecuteCliCommand(
		&inur,
		func() bool {
			return inur.VerificationToken != ""
		},
		"invite",
		email,
	)

	return inur.VerificationToken, err
}

func (c *Client) ResendInvite(email string) (string, error) {
	fmt.Printf("Re-inviting user: %v\n", email)

	var eur v1.EditUserReply
	err := c.ExecuteCliCommand(
		&eur,
		func() bool {
			return eur.VerificationToken != nil && *eur.VerificationToken != ""
		},
		"edituser",
		email,
		"resendinvite",
		"automatically sent by dataload util",
	)

	if eur.VerificationToken == nil {
		return "", err
	}
	return *eur.VerificationToken, err
}

func (c *Client) RegisterUser(email, username, password, token string) error {
	fmt.Printf("Registering user: %v\n", email)

	var rr v1.RegisterReply
	return c.ExecuteCliCommand(
		&rr,
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

func (c *Client) CreateAdminUser(email, username, password string) error {
	fmt.Printf("Creating admin user: %v\n", c.cfg.AdminEmail)
	_, err := c.ExecuteCommandWithErrorHandling(
		dbutil,
		"-testnet",
		"-createadmin",
		email,
		username,
		password,
	)
	return err
}

func (c *Client) DeleteAllData() error {
	_, err := c.ExecuteCommandWithErrorHandling(
		dbutil,
		"-testnet",
		"-deletedata",
		"i-understand-the-risks-of-this-action",
	)
	return err
}
