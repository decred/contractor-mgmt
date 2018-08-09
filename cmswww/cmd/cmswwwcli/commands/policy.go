package commands

import (
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type PolicyCmd struct{}

func (cmd *PolicyCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	var pr v1.PolicyReply
	return Ctx.Get(v1.RoutePolicy, nil, &pr)
}
