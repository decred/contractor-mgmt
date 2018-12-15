package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type UpdateInvoicePaymentCmd struct {
	Args struct {
		Token   string `positional-arg-name:"token"`
		Address string `positional-arg-name:"address"`
		Amount  uint   `positional-arg-name:"amount"`
		TxID    string `positional-arg-name:"txid"`
	} `positional-args:"true" required:"true"`
}

func (cmd *UpdateInvoicePaymentCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	uip := v1.UpdateInvoicePayment{
		Token:   cmd.Args.Token,
		Address: cmd.Args.Address,
		Amount:  cmd.Args.Amount,
		TxID:    cmd.Args.TxID,
	}

	var uipr v1.UpdateInvoicePaymentReply
	err = Ctx.Post(v1.RouteUpdateInvoicePayment, uip, &uipr)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoice payment has been updated\n")
	}

	return nil
}
