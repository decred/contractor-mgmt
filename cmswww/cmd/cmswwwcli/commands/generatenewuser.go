package commands

import (
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type GenerateNewUserCmd struct {
	Args struct {
		Email string `positional-arg-name:"email"`
	} `positional-args:"true" required:"true"`
}

func (cmd *GenerateNewUserCmd) Execute(args []string) error {
	gnu := v1.GenerateNewUser{
		Email: cmd.Args.Email,
	}

	var gnur v1.GenerateNewUserReply
	err := Ctx.Post(v1.RouteGenerateNewUser, gnu, &gnur)
	if err != nil {
		return err
	}

	return nil
}
