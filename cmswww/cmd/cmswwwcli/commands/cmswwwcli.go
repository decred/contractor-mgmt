package commands

import (
	"fmt"
	"strings"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/client"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type Options struct {
	// cli flags
	Host       func(string) error `long:"host" description:"cmswww host"`
	JSONOutput func()             `long:"jsonout" description:"Output only the last command's JSON output; use this option when writing scripts"`
	Verbose    func()             `short:"v" long:"verbose" description:"Print request and response details"`

	// cli commands
	InviteNewUser     InviteNewUserCmd  `command:"invite" description:"generate a new contractor invitation"`
	Login             LoginCmd          `command:"login" description:"login to the contractor mgmt system"`
	NewIdentity       NewIdentityCmd    `command:"newidentity" description:"generate a new identity"`
	VerifyNewIdentity VerifyIdentityCmd `command:"verifyidentity" description:"verify a newly generated identity"`
	Logout            LogoutCmd         `command:"logout" description:"logout of the contractor mgmt system"`
	Register          RegisterCmd       `command:"register" description:"complete registration as a contractor"`
	Policy            PolicyCmd         `command:"policy" description:"fetch server policy"`
	Version           VersionCmd        `command:"version" description:"fetch server info and CSRF token"`
}

var Ctx *client.Ctx
var Opts Options

func SetupOptsFunctions() {
	Opts.Host = func(host string) error {
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			return fmt.Errorf("host must begin with http:// or https://")
		}

		config.Host = host

		if err := config.LoadCsrf(); err != nil {
			return err
		}

		return config.LoadCookies()
	}

	Opts.JSONOutput = func() {
		config.JSONOutput = true
	}

	Opts.Verbose = func() {
		config.Verbose = true
	}
}
