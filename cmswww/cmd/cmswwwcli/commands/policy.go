package commands

import (
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type PolicyCmd struct{}

func fetchPolicy() (*v1.PolicyReply, error) {
	config.SuppressOutput = true
	defer func() {
		config.SuppressOutput = false
	}()

	var pr v1.PolicyReply
	err := Ctx.Get(v1.RoutePolicy, nil, &pr)
	return &pr, err
}

func (cmd *PolicyCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	var pr v1.PolicyReply
	return Ctx.Get(v1.RoutePolicy, nil, &pr)
}
