package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
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

func (c *Client) CreatePoliteiadCmd() *exec.Cmd {
	return c.ExecuteCommand(
		"politeiad",
		"--testnet",
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

	var stderrBytes, stdoutBytes []byte
	var errStderr, errStdout error
	go func() {
		stderrBytes, errStderr = ioutil.ReadAll(stderr)
		if errStderr == nil && len(stderrBytes) > 0 {
			errStderr = fmt.Errorf("unexpected error output from %v: %v", cli,
				string(stderrBytes))
		}
	}()

	go func() {
		stdoutBytes, errStdout = ioutil.ReadAll(stdout)
	}()

	err := cmd.Wait()
	if err != nil {
		return err
	}
	if errStderr != nil {
		return errStderr
	}
	if errStdout != nil {
		return errStdout
	}

	text := string(stdoutBytes)

	var lines []string
	if strings.Contains(text, "\n") {
		lines = strings.Split(text, "\n")
	} else {
		lines = append(lines, text)
	}

	var allText string
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

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

func (c *Client) Login(email, password string) (*v1.LoginReply, error) {
	fmt.Printf("Logging in as: %v\n", email)

	var lr v1.LoginReply
	err := c.ExecuteCliCommand(
		&lr,
		func() bool {
			return lr.UserID != ""
		},
		"login",
		email,
		password)

	return &lr, err
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

	var mur v1.ManageUserReply
	err := c.ExecuteCliCommand(
		&mur,
		func() bool {
			return mur.VerificationToken != nil && *mur.VerificationToken != ""
		},
		"manageuser",
		email,
		"resendinvite",
		"automatically sent by dataload util",
	)

	if mur.VerificationToken == nil {
		return "", err
	}
	return *mur.VerificationToken, err
}

func (c *Client) UserDetails(userID string) (*v1.UserDetailsReply, error) {
	fmt.Printf("Fetching user details\n")

	var udr v1.UserDetailsReply
	err := c.ExecuteCliCommand(
		&udr,
		func() bool {
			return udr.User.ID == userID
		},
		"user",
		userID,
	)

	return &udr, err
}

func (c *Client) EditUser(name, location, extendedPublicKey string) error {
	fmt.Printf("Editing user\n")

	var eur v1.EditUserReply
	err := c.ExecuteCliCommand(
		&eur,
		func() bool {
			return true
		},
		"edituser",
		fmt.Sprintf("--name=\"%v\"", name),
		fmt.Sprintf("--location=\"%v\"", location),
	)
	if err != nil {
		return err
	}

	var euepkr v1.EditUserExtendedPublicKeyReply
	return c.ExecuteCliCommand(
		&euepkr,
		func() bool {
			return true
		},
		"updatexpublickey",
		fmt.Sprintf("--xpublickey=\"%v\"", extendedPublicKey),
	)
}

func (c *Client) RegisterUser(
	email, username, password, name,
	location, extendedPublicKey, token string,
) error {
	fmt.Printf("Registering user: %v\n", email)

	var rr v1.RegisterReply
	return c.ExecuteCliCommand(
		&rr,
		func() bool {
			return true
		},
		"register",
		email,
		token,
		fmt.Sprintf("--username=\"%v\"", username),
		fmt.Sprintf("--password=\"%v\"", password),
		fmt.Sprintf("--name=\"%v\"", name),
		fmt.Sprintf("--location=\"%v\"", location),
		fmt.Sprintf("--xpubkey=%v", extendedPublicKey),
	)
}

func (c *Client) SubmitInvoice(file string) (string, error) {
	fmt.Printf("Submitting invoice\n")

	file = filepath.ToSlash(file)

	var sir v1.SubmitInvoiceReply
	err := c.ExecuteCliCommand(
		&sir,
		func() bool {
			return sir.CensorshipRecord.Token != ""
		},
		"submitinvoice",
		fmt.Sprintf("--invoice=%v", file),
	)

	return sir.CensorshipRecord.Token, err
}

func (c *Client) EditInvoice(token, file string) error {
	fmt.Printf("Editing invoice\n")

	file = filepath.ToSlash(file)

	var eir v1.EditInvoiceReply
	err := c.ExecuteCliCommand(
		&eir,
		func() bool {
			return eir.Invoice.CensorshipRecord.Token != ""
		},
		"editinvoice",
		token,
		file,
	)

	return err
}

func (c *Client) ApproveInvoice(token string) error {
	fmt.Printf("Approving invoice: %v\n", token)

	var sisr v1.SetInvoiceStatusReply
	return c.ExecuteCliCommand(
		&sisr,
		func() bool {
			return sisr.Invoice.Status == v1.InvoiceStatusApproved
		},
		"setinvoicestatus",
		token,
		"approved",
	)
}

func (c *Client) RejectInvoice(token, reason string) error {
	fmt.Printf("Rejecting invoice: %v\n", token)

	var sisr v1.SetInvoiceStatusReply
	return c.ExecuteCliCommand(
		&sisr,
		func() bool {
			return sisr.Invoice.Status == v1.InvoiceStatusRejected
		},
		"setinvoicestatus",
		token,
		"rejected",
		reason,
	)
}

func (c *Client) PayInvoices(month, year uint16, usdDCRRate float64) (*v1.PayInvoicesReply, error) {
	fmt.Printf("Paying invoices\n")

	var pir v1.PayInvoicesReply
	err := c.ExecuteCliCommand(
		&pir,
		func() bool {
			return true
		},
		"payinvoices",
		strconv.FormatUint(uint64(month), 10),
		strconv.FormatUint(uint64(year), 10),
		strconv.FormatFloat(usdDCRRate, 'f', -1, 64),
	)
	if err != nil {
		return nil, err
	}

	return &pir, nil
}

func (c *Client) PayInvoice(invoiceToken string, costUSD uint64, usdDCRRate float64) (*v1.PayInvoiceReply, error) {
	fmt.Printf("Paying invoice\n")

	var pir v1.PayInvoiceReply
	err := c.ExecuteCliCommand(
		&pir,
		func() bool {
			return true
		},
		"payinvoice",
		invoiceToken,
		strconv.FormatUint(costUSD, 10),
		strconv.FormatFloat(usdDCRRate, 'f', -1, 64),
	)
	if err != nil {
		return nil, err
	}

	return &pir, nil
}

func (c *Client) UpdateInvoicePayment(
	token, address string,
	amount uint64,
	txID string,
) (*v1.UpdateInvoicePaymentReply, error) {
	fmt.Printf("Updating invoice as paid\n")

	var uipr v1.UpdateInvoicePaymentReply
	err := c.ExecuteCliCommand(
		&uipr,
		func() bool {
			return true
		},
		"updateinvoicepayment",
		token,
		address,
		strconv.FormatUint(amount, 10),
		txID,
	)
	if err != nil {
		return nil, err
	}

	return &uipr, nil
}

func (c *Client) GetAllInvoices() ([]v1.InvoiceRecord, error) {
	fmt.Printf("Fetching all invoices\n")

	var ir v1.InvoicesReply
	err := c.ExecuteCliCommand(
		&ir,
		func() bool {
			return true
		},
		"invoices",
	)

	return ir.Invoices, err
}

func (c *Client) ChangePassword(currentPassword, newPassword string) error {
	fmt.Printf("Changing password to: %v\n", newPassword)

	var cpr v1.ChangePasswordReply
	return c.ExecuteCliCommand(
		&cpr,
		func() bool {
			return true
		},
		"changepassword",
		currentPassword,
		newPassword,
	)
}

func (c *Client) ResetPassword(email, newPassword string) error {
	fmt.Printf("Resetting password: %v\n", email)

	var rpr v1.ResetPasswordReply
	return c.ExecuteCliCommand(
		&rpr,
		func() bool {
			return true
		},
		"resetpassword",
		email,
		fmt.Sprintf("--newpassword=%v", newPassword),
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
