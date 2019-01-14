package commands

import (
	"fmt"
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type ManageUserCmd struct {
	Args struct {
		User   string `positional-arg-name:"user"`
		Action string `positional-arg-name:"action"`
		Reason string `positional-arg-name:"reason"`
	} `positional-args:"true" required:"true"`
}

var (
	UserManageActionCommands = map[string]v1.UserManageActionT{
		"resendinvite":        v1.UserManageResendInvite,
		"expireidentitytoken": v1.UserManageExpireUpdateIdentityVerification,
		"lock":                v1.UserManageLock,
		"unlock":              v1.UserManageUnlock,
	}
)

func (cmd *ManageUserCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	action, ok := UserManageActionCommands[cmd.Args.Action]
	if !ok {
		return fmt.Errorf("%v is an invalid user manage action", cmd.Args.Action)
	}

	mu := v1.ManageUser{
		UserID:   cmd.Args.User,
		Email:    cmd.Args.User,
		Username: cmd.Args.User,
		Action:   action,
		Reason:   cmd.Args.Reason,
	}

	var mur v1.ManageUserReply
	return Ctx.Post(v1.RouteManageUser, mu, &mur)
}
