package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type UserDetailsArgs struct {
	UserID string `positional-arg-name:"userid"`
}

type UserDetailsCmd struct {
	Args UserDetailsArgs `positional-args:"true" required:"true"`
}

func (cmd *UserDetailsCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	var udr v1.UserDetailsReply
	return Ctx.Get(fmt.Sprintf("/user/%v", cmd.Args.UserID), nil, &udr)
}
