package commands

import (
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/client"
)

type Options struct {
	GenerateNewUser GenerateNewUserCmd `command:"generatenewuser" description:"generate a new contractor invitation"`
	Login           LoginCmd           `command:"login" description:"login to the contractor mgmt system"`
	Logout          LogoutCmd          `command:"logout" description:"logout of the contractor mgmt system"`
	NewUser         NewUserCmd         `command:"newuser" description:"complete registration as a contractor"`
	Policy          PolicyCmd          `command:"policy" description:"fetch server policy"`
	Version         VersionCmd         `command:"version" description:"fetch server info and CSRF token"`
}

var Opts Options
var Ctx *client.Ctx
