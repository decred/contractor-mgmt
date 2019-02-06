package commands

import (
	"fmt"
	"log"

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
		log.Printf("Invoices to review: ")
		if len(rir.Invoices) == 0 {
			log.Printf("none\n")
		} else {
			for _, invoice := range rir.Invoices {
				fmt.Println()
				fmt.Println()

				totalRate := float64(invoice.TotalCostUSD) / float64(invoice.TotalHours)

				log.Printf("           User ID: %v\n", invoice.UserID)
				log.Printf("          Username: %v\n", invoice.Username)
				log.Printf("             Token: %v\n", invoice.Token)
				log.Printf("   ------------------------------------------\n")
				for lineItemIdx, lineItem := range invoice.LineItems {
					if lineItemIdx > 0 {
						log.Printf("        --------------------------------\n")
					}

					rate := float64(lineItem.TotalCost) / float64(lineItem.Hours)
					log.Printf("                 Type: %v\n", lineItem.Type)
					if lineItem.Subtype != "" {
						log.Printf("              Subtype: %v\n", lineItem.Subtype)
					}
					log.Printf("          Description: %v\n", lineItem.Description)
					if lineItem.Proposal != "" {
						log.Printf("    Politeia proposal: %v\n", lineItem.Proposal)
					}
					log.Printf("                Hours: %v\n", lineItem.Hours)
					log.Printf("           Total cost: $%v\n", lineItem.TotalCost)
					log.Printf("                 Rate: $%.2f / hr\n", rate)
				}
				log.Printf("   ------------------------------------------\n")
				log.Printf("             Hours: %v\n", invoice.TotalHours)
				log.Printf("        Total cost: $%v\n", invoice.TotalCostUSD)
				log.Printf("      Average Rate: $%.2f / hr\n", totalRate)
			}
		}
	}

	return nil
}
