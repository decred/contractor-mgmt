package commands

import (
	"log"

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
		log.Printf("                     ID: %v\n", udr.User.ID)
		log.Printf("                  Email: %v\n", udr.User.Email)
		log.Printf("               Username: %v\n", udr.User.Username)
		log.Printf("                  Admin: %v\n", udr.User.Admin)
		log.Printf("    Extended public key: %v\n", udr.User.ExtendedPublicKey)
		log.Printf("             Last login: %v\n", udr.User.LastLogin)
		log.Printf("  Failed login attempts: %v\n", udr.User.FailedLoginAttempts)
		log.Printf("                 Locked: %v\n",
			udr.User.FailedLoginAttempts >= v1.LoginAttemptsToLockUser)
	}

	return nil
}
