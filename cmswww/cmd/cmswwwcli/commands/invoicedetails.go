package commands

import (
	"fmt"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type InvoiceDetailsCmd struct {
	Args struct {
		Token string `positional-arg-name:"token"`
	} `positional-args:"true" optional:"true"`
}

func (cmd *InvoiceDetailsCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	id := v1.InvoiceDetails{
		Token: cmd.Args.Token,
	}

	var idr v1.InvoiceDetailsReply
	err = Ctx.Get(v1.RouteInvoiceDetails, id, &idr)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		date := time.Date(int(idr.Invoice.Year), time.Month(idr.Invoice.Month),
			1, 0, 0, 0, 0, time.UTC)
		fmt.Printf("%v %v\n", idr.Invoice.CensorshipRecord.Token[0:7],
			idr.Invoice.CensorshipRecord.Token)
		fmt.Printf("    Submitted by: %v\n", idr.Invoice.Username)
		fmt.Printf("              at: %v\n", time.Unix(idr.Invoice.Timestamp, 0))
		fmt.Printf("             For: %v\n", date.Format("January 2006"))

		var attachments string
		for idx, file := range idr.Invoice.Files {
			if idx == 0 {
				continue
			}

			if idx == 1 {
				attachments += fmt.Sprintf("%v  %v\n", idx, file.Name)
			} else {
				attachments += fmt.Sprintf("                  %v  %v\n", idx,
					file.Name)
			}
		}
		fmt.Printf("     Attachments: %v\n", attachments)
	}

	return nil
}
