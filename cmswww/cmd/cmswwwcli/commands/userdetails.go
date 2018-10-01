package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type UserDetailsArgs struct {
	User string `positional-arg-name:"user"`
}

type UserDetailsCmd struct {
	Args UserDetailsArgs `positional-args:"true" required:"true"`
}

func (cmd *UserDetailsCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	ud := v1.UserDetails{
		UserID:   cmd.Args.User,
		Email:    cmd.Args.User,
		Username: cmd.Args.User,
	}

	var udr v1.UserDetailsReply
	err = Ctx.Get(v1.RouteUserDetails, ud, &udr)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("                     ID: %v\n", udr.User.ID)
		fmt.Printf("                  Email: %v\n", udr.User.Email)
		fmt.Printf("               Username: %v\n", udr.User.Username)
		fmt.Printf("                  Admin: %v\n", udr.User.Admin)
		fmt.Printf("    Extended public key: %v\n", udr.User.ExtendedPublicKey)
		fmt.Printf("             Last login: %v\n", udr.User.LastLogin)
		fmt.Printf("  Failed login attempts: %v\n", udr.User.FailedLoginAttempts)
		fmt.Printf("                 Locked: %v\n",
			udr.User.FailedLoginAttempts >= v1.LoginAttemptsToLockUser)
	}

	return nil
}
