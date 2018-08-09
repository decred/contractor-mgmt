package commands

import (
	"fmt"
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type EditUserCmd struct {
	Args struct {
		UserID string `positional-arg-name:"userid"`
		Action string `positional-arg-name:"action"`
		Reason string `positional-arg-name:"reason"`
	} `positional-args:"true" required:"true"`
}

var (
	UserEditActionCommands = map[string]v1.UserEditActionT{
		"resendinvite":        v1.UserEditResendInvite,
		"resendidentitytoken": v1.UserEditRegenerateUpdateIdentityVerification,
		"lock":                v1.UserEditLock,
		"unlock":              v1.UserEditUnlock,
	}
)

func (cmd *EditUserCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	action, ok := UserEditActionCommands[cmd.Args.Action]
	if !ok {
		return fmt.Errorf("%v is an invalid user edit action", cmd.Args.Action)
	}

	eu := v1.EditUser{
		UserID: cmd.Args.UserID,
		Action: action,
		Reason: cmd.Args.Reason,
	}

	var eur v1.EditUserReply
	return Ctx.Post(v1.RouteEditUser, eu, &eur)
}
