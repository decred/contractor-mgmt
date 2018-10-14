package commands

import (
	"fmt"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type ReviewInvoicesCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" required:"true"`
}

func (cmd *ReviewInvoicesCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	month, err := ParseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}

	ri := v1.ReviewInvoices{
		Month: month,
		Year:  cmd.Args.Year,
	}

	var rir v1.ReviewInvoicesReply
	err = Ctx.Post(v1.RouteReviewInvoices, ri, &rir)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Printf("Invoices to review: ")
		if len(rir.Invoices) == 0 {
			fmt.Printf("none\n")
		} else {
			for _, invoice := range rir.Invoices {
				fmt.Println()
				fmt.Println()

				totalRate := float64(invoice.TotalCostUSD) / float64(invoice.TotalHours)

				fmt.Printf("           User ID: %v\n", invoice.UserID)
				fmt.Printf("          Username: %v\n", invoice.Username)
				fmt.Printf("             Token: %v\n", invoice.Token)
				fmt.Printf("   ------------------------------------------\n")
				for lineItemIdx, lineItem := range invoice.LineItems {
					if lineItemIdx > 0 {
						fmt.Printf("        --------------------------------\n")
					}

					rate := float64(lineItem.TotalCost) / float64(lineItem.Hours)
					fmt.Printf("                 Type: %v\n", lineItem.Type)
					if lineItem.Subtype != "" {
						fmt.Printf("              Subtype: %v\n", lineItem.Subtype)
					}
					fmt.Printf("          Description: %v\n", lineItem.Description)
					if lineItem.Proposal != "" {
						fmt.Printf("    Politeia proposal: %v\n", lineItem.Proposal)
					}
					fmt.Printf("                Hours: %v\n", lineItem.Hours)
					fmt.Printf("           Total cost: $%v\n", lineItem.TotalCost)
					fmt.Printf("                 Rate: $%.2f / hr\n", rate)
				}
				fmt.Printf("   ------------------------------------------\n")
				fmt.Printf("             Hours: %v\n", invoice.TotalHours)
				fmt.Printf("        Total cost: $%v\n", invoice.TotalCostUSD)
				fmt.Printf("      Average Rate: $%.2f / hr\n", totalRate)
			}
		}
	}

	return nil
}
