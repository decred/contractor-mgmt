package commands

import (
	"log"
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
		log.Printf("%v %v\n", idr.Invoice.CensorshipRecord.Token[0:7],
			idr.Invoice.CensorshipRecord.Token)
		log.Printf("    Submitted by: %v\n", idr.Invoice.Username)
		log.Printf("              at: %v\n", time.Unix(idr.Invoice.Timestamp, 0))
		log.Printf("             For: %v\n", date.Format("January 2006"))
	}

	return nil
}
