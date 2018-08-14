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

var (
	invoiceStatuses = map[string]v1.InvoiceStatusT{
		"unreviewed": v1.InvoiceStatusNotReviewed,
		"rejected":   v1.InvoiceStatusRejected,
		"approved":   v1.InvoiceStatusApproved,
	}
)

func (cmd *InvoicesCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	status, ok := invoiceStatuses[strings.ToLower(cmd.Args.Status)]
	if !ok {
		return fmt.Errorf("Invalid status: %v", cmd.Args.Status)
	}

	month, err := parseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}

	i := v1.Invoices{
		Status: status,
		Month:  month,
		Year:   cmd.Args.Year,
	}

	var ir v1.InvoicesReply
	err = Ctx.Get(v1.RouteInvoices, i, &ir)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoices: ")
		if len(ir.Invoices) == 0 {
			fmt.Printf("none\n")
		} else {
			fmt.Println()
			for _, v := range ir.Invoices {
				fmt.Printf("  %v %v\n", v.CensorshipRecord.Token[0:7], v.CensorshipRecord.Token)
				fmt.Printf("      Submitted by: %v\n", v.Username)
				fmt.Printf("                at: %v\n",
					time.Unix(v.Timestamp, 0).String())
			}
		}
	}

	return nil
}
