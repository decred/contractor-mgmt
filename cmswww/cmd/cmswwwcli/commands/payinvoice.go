package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type PayInvoiceCmd struct {
	Args struct {
		Token      string  `positional-arg-name:"token"`
		CostUSD    uint64  `positional-arg-name:"costusd"`
		USDDCRRate float64 `positional-arg-name:"usddcrrate"`
	} `positional-args:"true" required:"true"`
}

func (cmd *PayInvoiceCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	pi := v1.PayInvoice{
		Token:      cmd.Args.Token,
		CostUSD:    cmd.Args.CostUSD,
		USDDCRRate: cmd.Args.USDDCRRate,
	}

	var pir v1.PayInvoiceReply
	err = Ctx.Post(v1.RoutePayInvoice, pi, &pir)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		invoice := pir.Invoice

		fmt.Printf("           User ID: %v\n", invoice.UserID)
		fmt.Printf("          Username: %v\n", invoice.Username)
		fmt.Printf("     Invoice token: %v\n", invoice.Token)
		fmt.Printf("   ------------------------------------------\n")
		fmt.Printf("        Total cost: $%v\n", invoice.TotalCostUSD)
		fmt.Printf("                    %v DCR\n", invoice.TotalCostDCR)
		fmt.Printf("   Payment Address: %v\n", invoice.PaymentAddress)
	}

	return nil
}
