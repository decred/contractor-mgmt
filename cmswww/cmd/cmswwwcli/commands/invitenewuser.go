package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type InviteNewUserCmd struct {
	Args struct {
		Email string `positional-arg-name:"email"`
	} `positional-args:"true" required:"true"`
}

func (cmd *InviteNewUserCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	gnu := v1.InviteNewUser{
		Email: cmd.Args.Email,
	}

	var gnur v1.InviteNewUserReply
	err = Ctx.Post(v1.RouteInviteNewUser, gnu, &gnur)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invitation created and sent to %v. Their invitation will expire"+
			" in %v.\n", cmd.Args.Email, v1.VerificationExpiryTime.String())
	}
	return nil
}
