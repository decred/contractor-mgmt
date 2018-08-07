package commands

import (
	"encoding/hex"

	"github.com/decred/politeia/politeiad/api/v1/identity"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type NewUserArgs struct {
	Email    string `positional-arg-name:"email"`
	Username string `positional-arg-name:"username"`
	Password string `positional-arg-name:"password"`
	Token    string `positional-arg-name:"token"`
}

type NewUserCmd struct {
	Args NewUserArgs `positional-args:"true" required:"true"`
}

func (cmd *NewUserCmd) Execute(args []string) error {
	id, err := identity.New()
	if err != nil {
		return err
	}
	err = config.SaveUserIdentity(id, cmd.Args.Email)
	if err != nil {
		return err
	}

	signature := id.SignMessage([]byte(cmd.Args.Token))

	nu := v1.NewUser{
		Email:             cmd.Args.Email,
		Username:          cmd.Args.Username,
		Password:          cmd.Args.Password,
		VerificationToken: cmd.Args.Token,
		PublicKey:         hex.EncodeToString(id.Public.Key[:]),
		Signature:         hex.EncodeToString(signature[:]),
	}

	var nur v1.NewUserReply
	err = Ctx.Post(v1.RouteNewUser, nu, &nur)
	if err != nil {
		config.DeleteUserIdentity(nu.Email)
	}

	return err
}
