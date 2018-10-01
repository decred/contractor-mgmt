package commands

import (
	"fmt"
	"github.com/decred/contractor-mgmt/cmswww/api/v1"
)

type UpdateExtendedPublicKeyCmd struct {
	Token             string `long:"token" optional:"true" description:"Verification token"`
	ExtendedPublicKey string `long:"xpublickey" optional:"true" description:"User's extended public key for payment account"`
}

func (cmd *UpdateExtendedPublicKeyCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	// The update extended public key command is special.  It must be called
	// twice with different parameters.  For the 1st call, it should be called
	// with no parameters. On success it will send an email containing a
	// verification token to the email address provided.  If the email server
	// has been disabled, the verification token is sent back in the response
	// body. The 2nd call should be called with a verification token and new
	// extended public key parameters.
	//
	// If the email server is disabled and the server sends back the verification
	// token, politeiawwwcli will automatically verify the token for the user.

	eu := &v1.EditUserExtendedPublicKey{}

	if cmd.Token != "" {
		eu.VerificationToken = cmd.Token
		eu.ExtendedPublicKey = cmd.ExtendedPublicKey
	}

	var eur v1.EditUserExtendedPublicKeyReply
	err = Ctx.Post(v1.RouteEditUserExtendedPublicKey, eu, &eur)
	if err != nil {
		return err
	}

	if eur.VerificationToken != "" {
		if cmd.ExtendedPublicKey != "" {
			// Automatic 2nd extended public key call
			eu = &v1.EditUserExtendedPublicKey{
				ExtendedPublicKey: cmd.ExtendedPublicKey,
				VerificationToken: eur.VerificationToken,
			}

			eur = v1.EditUserExtendedPublicKeyReply{}
			return Ctx.Post(v1.RouteEditUserExtendedPublicKey, eu, &eur)
		}

		// The verification token was returned by the server, but the user
		// did not provide the password in the call, so throw an error.
		return fmt.Errorf("The server is running with email disabled, the"+
			" verification token is: %v", eur.VerificationToken)
	}

	return nil
}
