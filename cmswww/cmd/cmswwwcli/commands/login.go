package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type LoginCmd struct {
	Args struct {
		Email    string `positional-arg-name:"email"`
		Password string `positional-arg-name:"password"`
	} `positional-args:"true" required:"true"`
}

func (cmd *LoginCmd) Execute(args []string) error {
	l := v1.Login{
		Email:    cmd.Args.Email,
		Password: cmd.Args.Password,
	}

	var lr v1.LoginReply
	err := Ctx.Post(v1.RouteLogin, l, &lr)
	if err != nil {
		return err
	}

	config.LoggedInUser = &lr
	if !config.JSONOut {
		fmt.Printf("\nYou are now logged in as %v\n", lr.Username)
	}

	// Load identity, if available.
	_, err = config.LoadUserIdentity(cmd.Args.Email)
	if err != nil && !config.JSONOut {
		fmt.Printf("WARNING: Your identity could not be loaded, please generate" +
			" a new one using the newidentity command\n")
	}

	fmt.Println()
	/*
		   is this necessary?!

		   	// persist CSRF header token
		   	err = config.SaveCsrf(Ctx.Csrf)
		   	if err != nil {
		   		return err
		   	}

		// persist session cookie
		ck, err := Ctx.Cookies(Ctx.Config.Host)
		if err != nil {
			return err
		}
		err = config.SaveCookies(ck)
		return err
	*/

	return nil
}
