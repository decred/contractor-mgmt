package commands

import (
	"log"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type UsersCmd struct {
	Username string `long:"username" optional:"true" description:"Username"`
}

func (cmd *UsersCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	u := v1.Users{
		Username: cmd.Username,
	}

	var ur v1.UsersReply
	err = Ctx.Get(v1.RouteUsers, u, &ur)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		log.Printf("Displaying %v / %v matches\n", len(ur.Users), ur.TotalMatches)
		log.Printf("---------------------------\n")
		for _, user := range ur.Users {
			log.Printf("       ID: %v\n", user.ID)
			log.Printf("    Email: %v\n", user.Email)
			log.Printf(" Username: %v\n", user.Username)
			log.Printf("    Admin: %v\n", user.Admin)
			log.Printf("---------------------------\n")
		}
	}

	return nil
}
