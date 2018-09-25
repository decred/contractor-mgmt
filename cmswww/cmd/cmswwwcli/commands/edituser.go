package commands

import (
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type EditUserCmd struct {
	Name              *string `long:"name" optional:"true" description:"User's full name"`
	Location          *string `long:"location" optional:"true" description:"User's physical location"`
	ExtendedPublicKey *string `long:"xpubkey" optional:"true" description:"User's extended public key for payment account"`
}

func (cmd *EditUserCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	eu := v1.EditUser{
		Name:              cmd.Name,
		Location:          cmd.Location,
		ExtendedPublicKey: cmd.ExtendedPublicKey,
	}

	var eur v1.EditUserReply
	return Ctx.Post(v1.RouteEditUser, eu, &eur)
}
