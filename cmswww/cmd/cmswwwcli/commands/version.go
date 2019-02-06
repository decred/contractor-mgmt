package commands

import (
	"log"

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

	config.ServerPublicKey = vr.PublicKey

	if vr.User != nil {
		config.LoggedInUser = vr.User
		if !config.JSONOutput && !config.SuppressOutput {
			log.Printf("You are currently logged in as %v\n", vr.User.Username)
		}

		// Load identity, if available.
		_, err = config.LoadUserIdentity(vr.User.Email)
		if err != nil && !config.JSONOutput {
			log.Printf("WARNING: Your identity could not be loaded, please generate" +
				" a new one using the newidentity command\n")
		}
	} else if !config.JSONOutput && !config.SuppressOutput {
		log.Printf("You are not currently logged in\n")
	}

	// CSRF protection works via double-submit method. One token is stored in the
	// cookie. A different token is sent via the header. Both tokens must be
	// persisted between cli commands.

	// persist session cookie
	ck, err := Ctx.Cookies(config.Host)
	if err != nil {
		return err
	}
	return config.SaveCookies(ck)
}

func InitialVersionRequest() error {
	config.SuppressOutput = true
	defer func() {
		config.SuppressOutput = false
	}()

	version := VersionCmd{}
	return version.Execute(nil)
}
