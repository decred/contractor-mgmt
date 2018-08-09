package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type LogoutCmd struct{}

func (cmd *LogoutCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	err = Ctx.Post(v1.RouteLogout, v1.Logout{}, nil)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("You are now logged out\n")
	}
	config.LoggedInUser = nil
	config.LoggedInUserIdentity = nil

	return config.DeleteCookies()
}
