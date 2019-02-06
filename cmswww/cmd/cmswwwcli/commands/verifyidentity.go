package commands

import (
	"encoding/hex"
	"log"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type VerifyIdentityCmd struct {
	Args struct {
		Token string `positional-arg-name:"token"`
	} `positional-args:"true" required:"true"`
}

func (cmd *VerifyIdentityCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	if config.LoggedInUser == nil {
		log.Printf("You must be logged in to perform this action.\n")
		return nil
	}

	id, err := config.LoadUserIdentity(config.LoggedInUser.Email)
	if err != nil {
		return err
	}

	signature := id.SignMessage([]byte(cmd.Args.Token))
	vni := v1.VerifyNewIdentity{
		VerificationToken: cmd.Args.Token,
		Signature:         hex.EncodeToString(signature[:]),
	}

	var vnir v1.VerifyNewIdentityReply
	return Ctx.Post(v1.RouteVerifyNewIdentity, vni, &vnir)
}
