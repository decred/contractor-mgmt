package commands

import (
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type EditUserCmd struct {
	Name               *string `long:"name" optional:"true" description:"User's full name"`
	Location           *string `long:"location" optional:"true" description:"User's physical location"`
	EmailNotifications *uint64 `long:"emailnotifications" optional:"true" description:"Whether to notify via emails"`
}

func (cmd *EditUserCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	eu := v1.EditUser{
		Name:               cmd.Name,
		Location:           cmd.Location,
		EmailNotifications: cmd.EmailNotifications,
	}

	var eur v1.EditUserReply
	return Ctx.Post(v1.RouteEditUser, eu, &eur)
}
