package commands

import (
	"fmt"
	"github.com/decred/contractor-mgmt/cmswww/api/v1"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type VersionCmd struct{}

func (cmd *VersionCmd) Execute(args []string) error {
	var vr v1.VersionReply
	err := Ctx.Get(v1.RouteRoot, nil, &vr)
	if err != nil {
		return err
	}

	if vr.User != nil {
		config.LoggedInUser = vr.User
		fmt.Printf("You are logged in as %v\n", vr.User.Username)
	}

	// CSRF protection works via double-submit method. One token is stored in the
	// cookie. A different token is sent via the header. Both tokens must be
	// persisted between cli commands.

	// persist CSRF header token
	err = config.SaveCsrf(Ctx.Csrf)
	if err != nil {
		return err
	}

	// persist session cookie
	ck, err := Ctx.Cookies(config.Host)
	if err != nil {
		return err
	}
	err = config.SaveCookies(ck)

	return err
}
