package commands

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/decred/politeia/politeiad/api/v1/identity"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type RegisterCmd struct {
	Args struct {
		Email string `positional-arg-name:"email"`
		Token string `positional-arg-name:"token"`
	} `positional-args:"true" required:"true"`
	Username          string `long:"username" optional:"true" description:"Username"`
	Password          string `long:"password" optional:"true" description:"Password"`
	Name              string `long:"name" optional:"true" description:"Full name"`
	Location          string `long:"location" optional:"true" description:"Location (e.g. Dallas, TX, USA)"`
	ExtendedPublicKey string `long:"xpubkey" optional:"true" description:"The extended public key for the payment account"`
}

func (cmd *RegisterCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	if cmd.Username == "" || cmd.Password == "" || cmd.Name == "" ||
		cmd.Location == "" || cmd.ExtendedPublicKey == "" {
		reader := bufio.NewReader(os.Stdin)
		if cmd.Username == "" {
			fmt.Print("Create a username: ")
			cmd.Username, _ = reader.ReadString('\n')
		}
		if cmd.Password == "" {
			fmt.Print("Create a password: ")
			cmd.Password, _ = reader.ReadString('\n')
		}
		if cmd.Name == "" {
			fmt.Print("Enter your full name: ")
			cmd.Name, _ = reader.ReadString('\n')
		}
		if cmd.Location == "" {
			fmt.Print("Enter your location (e.g. Dallas, TX, USA): ")
			cmd.Location, _ = reader.ReadString('\n')
		}
		if cmd.ExtendedPublicKey == "" {
			fmt.Print("Enter the extended public key for your payment account: ")
			cmd.ExtendedPublicKey, _ = reader.ReadString('\n')
		}

		fmt.Print("\nPlease carefully review your information and ensure it's " +
			"correct. If not, press Ctrl + C to exit. Or, press Enter to continue " +
			"your registration.")
		reader.ReadString('\n')
	}

	id, err := identity.New()
	if err != nil {
		return err
	}
	err = config.SaveUserIdentity(id, cmd.Args.Email)
	if err != nil {
		return err
	}

	signature := id.SignMessage([]byte(cmd.Args.Token))

	nu := v1.Register{
		Email:             cmd.Args.Email,
		Username:          cmd.Username,
		Password:          DigestSHA3(cmd.Password),
		Name:              cmd.Name,
		Location:          cmd.Location,
		ExtendedPublicKey: cmd.ExtendedPublicKey,
		VerificationToken: cmd.Args.Token,
		PublicKey:         hex.EncodeToString(id.Public.Key[:]),
		Signature:         hex.EncodeToString(signature[:]),
	}

	var nur v1.RegisterReply
	err = Ctx.Post(v1.RouteRegister, nu, &nur)
	if err != nil {
		config.DeleteUserIdentity(nu.Email)
	}

	return err
}
