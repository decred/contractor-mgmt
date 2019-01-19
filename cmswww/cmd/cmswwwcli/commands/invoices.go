package commands

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type InvoicesCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" optional:"required"`
	Status string `long:"status" optional:"true" description:"Invoice status"`
}

var (
	invoiceStatuses = map[string]v1.InvoiceStatusT{
		"unreviewed": v1.InvoiceStatusNotReviewed,
		"rejected":   v1.InvoiceStatusRejected,
		"approved":   v1.InvoiceStatusApproved,
		"paid":       v1.InvoiceStatusPaid,
	}
)

func (cmd *InvoicesCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	var status v1.InvoiceStatusT
	if cmd.Status != "" {
		var ok bool
		status, ok = invoiceStatuses[strings.ToLower(cmd.Status)]
		if !ok {
			return fmt.Errorf("Invalid status: %v", cmd.Status)
		}
	}

	month, err := ParseMonth(cmd.Args.Month)
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
		log.Printf("Invoices: ")
		if len(ir.Invoices) == 0 {
			log.Printf("none\n")
		} else {
			fmt.Println()
			for _, v := range ir.Invoices {
				log.Printf("  %v\n", v.CensorshipRecord.Token)
				log.Printf("      Submitted by: %v\n", v.Username)
				log.Printf("                at: %v\n",
					time.Unix(v.Timestamp, 0).String())
				if cmd.Status == "" {
					log.Printf("            Status: %v\n",
						v1.InvoiceStatus[v.Status])
				}
			}
		}
	}

	return nil
}
