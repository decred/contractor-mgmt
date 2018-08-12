package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type InvoicesCmd struct {
	Args struct {
		Status string `positional-arg-name:"status"`
		Month  string `positional-arg-name:"month"`
		Year   uint16 `positional-arg-name:"year"`
	} `positional-args:"true" optional:"true"`
}

func (cmd *InvoicesCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	month, err := parseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}

	status := strings.ToLower(cmd.Args.Status)
	if status == "unreviewed" {
		ui := v1.UnreviewedInvoices{
			Month: month,
			Year:  cmd.Args.Year,
		}

		var uir v1.UnreviewedInvoicesReply
		err = Ctx.Get(v1.RouteUnreviewedInvoices, ui, &uir)
		if err != nil {
			return err
		}

		if !config.JSONOutput {
			fmt.Printf("Unreviewed invoices: ")
			if len(uir.Invoices) == 0 {
				fmt.Printf("none\n")
			} else {
				fmt.Println()
				for _, v := range uir.Invoices {
					fmt.Printf("  %v\n", v.CensorshipRecord.Token)
					fmt.Printf("      Submitted by: %v\n", v.Username)
					fmt.Printf("                at: %v\n",
						time.Unix(v.Timestamp, 0).String())
				}
			}
		}
	}

	return nil
}
