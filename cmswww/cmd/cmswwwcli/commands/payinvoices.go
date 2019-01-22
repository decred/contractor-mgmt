package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type PayInvoicesCmd struct {
	Args struct {
		Month      string  `positional-arg-name:"month"`
		Year       uint16  `positional-arg-name:"year"`
		USDDCRRate float64 `positional-arg-name:"usddcrrate"`
	} `positional-args:"true" required:"true"`
}

func (cmd *PayInvoicesCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	month, err := ParseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}

	pi := v1.PayInvoices{
		Month:      month,
		Year:       cmd.Args.Year,
		USDDCRRate: cmd.Args.USDDCRRate,
	}

	var pir v1.PayInvoicesReply
	err = Ctx.Post(v1.RoutePayInvoices, pi, &pir)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoices ready to be paid: ")
		if len(pir.Invoices) == 0 {
			fmt.Printf("none\n")
		} else {
			for _, invoice := range pir.Invoices {
				fmt.Println()
				fmt.Println()

				rate := float64(invoice.TotalCostUSD) / float64(invoice.TotalHours)

				fmt.Printf("           User ID: %v\n", invoice.UserID)
				fmt.Printf("          Username: %v\n", invoice.Username)
				fmt.Printf("     Invoice token: %v\n", invoice.Token)
				fmt.Printf("   ------------------------------------------\n")
				fmt.Printf("             Hours: %v\n", invoice.TotalHours)
				fmt.Printf("        Total cost: $%v\n", invoice.TotalCostUSD)
				fmt.Printf("      Average Rate: $%.2f / hr\n", rate)
				fmt.Printf("   ------------------------------------------\n")
				fmt.Printf("        Total cost: %v DCR\n", invoice.TotalCostDCR)
				fmt.Printf("   Payment Address: %v\n", invoice.PaymentAddress)
			}
		}
	}

	return nil
}
