package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type ResetPasswordCmd struct {
	Args struct {
		Email string `positional-arg-name:"email"`
	} `positional-args:"true" required:"true"`
	Token       string `long:"token" optional:"true" description:"Verification token"`
	NewPassword string `long:"newpassword" optional:"true" description:"Your new password"`
}

func (cmd *ResetPasswordCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	// Get the password requirements.
	policy, err := fetchPolicy()
	if err != nil {
		return err
	}

	if cmd.Token != "" {
		// Validate the new password.
		if uint(len(cmd.NewPassword)) < policy.MinPasswordLength {
			return fmt.Errorf("Password must be %v characters long",
				policy.MinPasswordLength)
		}
	}

	// The reset password command is special.  It must be called twice with
	// different parameters.  For the 1st call, it should be called with only
	// the email parameter. On success it will send an email containing a
	// verification token to the email address provided.  If the email server
	// has been disabled, the verification token is sent back in the response
	// body. The 2nd call to reset password should be called with an email,
	// verification token, and new password parameters.
	//
	// If the email server is disabled and the server sends back the verification
	// token, politeiawwwcli will automatically verify the token for the user.

	rp := &v1.ResetPassword{
		Email: cmd.Args.Email,
	}

	if cmd.Token != "" {
		rp.VerificationToken = cmd.Token
		rp.NewPassword = cmd.NewPassword
	}

	var rpr v1.ResetPasswordReply
	err = Ctx.Post(v1.RouteResetPassword, rp, &rpr)
	if err != nil {
		return err
	}

	if rpr.VerificationToken != "" {
		if cmd.NewPassword != "" {
			// Automatic 2nd reset password call
			rp = &v1.ResetPassword{
				Email:             cmd.Args.Email,
				NewPassword:       cmd.NewPassword,
				VerificationToken: rpr.VerificationToken,
			}

			rpr = v1.ResetPasswordReply{}
			return Ctx.Post(v1.RouteResetPassword, rp, &rpr)
		}

		// The verification token was returned by the server, but the user
		// did not provide the password in the call, so throw an error.
		return fmt.Errorf("The server is running with email disabled, the"+
			" verification token is: %v", rpr.VerificationToken)
	}

	return nil
}
