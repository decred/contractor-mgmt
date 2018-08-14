package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type MyInvoicesCmd struct {
	Args struct {
		Status string `positional-arg-name:"status"`
	} `positional-args:"true" optional:"true"`
}

func (cmd *MyInvoicesCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	var status v1.InvoiceStatusT
	if cmd.Args.Status != "" {
		var ok bool
		status, ok = invoiceStatuses[strings.ToLower(cmd.Args.Status)]
		if !ok {
			return fmt.Errorf("Invalid status: %v", cmd.Args.Status)
		}
	}

	mi := v1.MyInvoices{
		Status: status,
	}

	var mir v1.MyInvoicesReply
	err = Ctx.Get(v1.RouteUserInvoices, mi, &mir)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoices: ")
		if len(mir.Invoices) == 0 {
			fmt.Printf("none\n")
		} else {
			fmt.Println()
			for _, v := range mir.Invoices {
				date := time.Date(int(v.Year), time.Month(v.Month),
					1, 0, 0, 0, 0, time.UTC)
				fmt.Printf("  %v\n", v.CensorshipRecord.Token)
				fmt.Printf("      Submitted at: %v\n",
					time.Unix(v.Timestamp, 0).String())
				fmt.Printf("               For: %v\n", date.Format("January 2006"))
				if cmd.Args.Status == "" {
					fmt.Printf("            Status: %v\n",
						v1.InvoiceStatus[v.Status])
				}
			}
		}
	}

	return nil
}
