package commands

import (
	"encoding/hex"
	"fmt"

	"github.com/decred/politeia/politeiad/api/v1/identity"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type NewIdentityCmd struct{}

func (cmd *NewIdentityCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	if config.LoggedInUser == nil {
		fmt.Printf("You must be logged in to perform this action.\n")
	}

	// Generate and save the new identity.
	id, err := identity.New()
	if err != nil {
		return err
	}
	err = config.SaveUserIdentity(id, config.LoggedInUser.Email)
	if err != nil {
		return err
	}

	ni := v1.NewIdentity{
		PublicKey: hex.EncodeToString(id.Public.Key[:]),
	}

	var nir v1.NewIdentityReply
	return Ctx.Post(v1.RouteNewIdentity, ni, &nir)
}
