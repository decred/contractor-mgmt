package commands

import (
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type SetInvoiceStatusCmd struct {
	Args struct {
		Token  string  `positional-arg-name:"token"`
		Status string  `positional-arg-name:"status"`
		Reason *string `positional-arg-name:"reason"`
	} `positional-args:"true" optional:"true"`
}

func (cmd *SetInvoiceStatusCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	id := config.LoggedInUserIdentity
	if id == nil {
		return ErrNotLoggedIn
	}

	status, ok := invoiceStatuses[strings.ToLower(cmd.Args.Status)]
	if !ok {
		return fmt.Errorf("Invalid status: %v", cmd.Args.Status)
	}

	msg := cmd.Args.Token + strconv.FormatUint(uint64(status), 10)
	signature := id.SignMessage([]byte(msg))

	sis := v1.SetInvoiceStatus{
		Token:     cmd.Args.Token,
		Status:    status,
		Reason:    cmd.Args.Reason,
		PublicKey: hex.EncodeToString(id.Public.Key[:]),
		Signature: hex.EncodeToString(signature[:]),
	}

	var sisr v1.SetInvoiceStatusReply
	err = Ctx.Post(v1.RouteSetInvoiceStatus, sis, &sisr)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		log.Printf("Status changed to %v", v1.InvoiceStatus[sisr.Invoice.Status])
	}

	return nil
}
