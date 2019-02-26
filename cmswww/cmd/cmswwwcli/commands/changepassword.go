package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type ChangePasswordCmd struct {
	Args struct {
		CurrentPassword string `positional-arg-name:"currentpassword"`
		NewPassword     string `positional-arg-name:"newpassword"`
	} `positional-args:"true" required:"true"`
}

func (cmd *ChangePasswordCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	// Get the password requirements.
	policy, err := fetchPolicy()
	if err != nil {
		return err
	}

	// Validate the new password.
	if uint(len(cmd.Args.NewPassword)) < policy.MinPasswordLength {
		return fmt.Errorf("Password must be %v characters long",
			policy.MinPasswordLength)
	}

	// Send the change password request.
	cp := v1.ChangePassword{
		CurrentPassword: DigestSHA3(cmd.Args.CurrentPassword),
		NewPassword:     DigestSHA3(cmd.Args.NewPassword),
	}

	var cpr v1.ChangePasswordReply
	return Ctx.Post(v1.RouteChangePassword, cp, &cpr)
}
