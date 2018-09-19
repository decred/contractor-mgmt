package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

type invoices struct {
	Invoices []invoice `json:"invoices"`
}

type invoice struct {
	Token     *string `json:"token"`
	Timestamp *int64  `json:"timestamp"`
	Month     uint16  `json:"month"`
	Year      uint16  `json:"year"`
	Status    string  `json:"status"`
}

type sortableInvoices []invoice

func (si sortableInvoices) Len() int {
	return len(si)
}

func (si sortableInvoices) Swap(i, j int) {
	si[i], si[j] = si[j], si[i]
}

func (si sortableInvoices) Less(i, j int) bool {
	if si[i].Year > si[j].Year {
		return true
	}
	if si[i].Year == si[j].Year {
		if si[i].Month > si[j].Month {
			return true
		}
	}

	return false
}

func fetchSubmittedInvoices(statusStr string) ([]invoice, error) {
	var status v1.InvoiceStatusT
	if statusStr != "" {
		var ok bool
		status, ok = invoiceStatuses[statusStr]
		if !ok {
			return nil, fmt.Errorf("Invalid status: %v", statusStr)
		}
	}

	mi := v1.MyInvoices{
		Status: status,
	}

	var mir v1.MyInvoicesReply
	err := Ctx.Get(v1.RouteUserInvoices, mi, &mir)
	if err != nil {
		return nil, err
	}

	invoices := make([]invoice, 0, len(mir.Invoices))
	for _, v := range mir.Invoices {
		// Create local variables to avoid pointer sharing of the range
		// variable.
		token := v.CensorshipRecord.Token
		timestamp := v.Timestamp

		invoices = append(invoices, invoice{
			Token:     &token,
			Month:     v.Month,
			Year:      v.Year,
			Status:    v1.InvoiceStatus[v.Status],
			Timestamp: &timestamp,
		})
	}

	return invoices, nil
}

func fetchUnsubmittedInvoices(submittedInvoices []invoice) ([]invoice, error) {
	var unsubmittedInvoices []invoice
	dirpath := config.GetInvoiceDirectory()
	err := filepath.Walk(dirpath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		filename := info.Name()
		year, month, err := config.GetMonthAndYearFromInvoice(filename)
		if err == nil {
			for _, v := range submittedInvoices {
				if v.Month == month && v.Year == year {
					return nil
				}
			}

			unsubmittedInvoices = append(unsubmittedInvoices, invoice{
				Month:  month,
				Year:   year,
				Status: "unsubmitted",
			})
		}

		return nil
	})

	return unsubmittedInvoices, err
}

func (cmd *MyInvoicesCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	if config.LoggedInUser == nil {
		return ErrNotLoggedIn
	}

	statusStr := strings.ToLower(cmd.Args.Status)

	var submittedInvoices []invoice
	var unsubmittedInvoices []invoice
	if statusStr != "unsubmitted" {
		submittedInvoices, err = fetchSubmittedInvoices(statusStr)
		if err != nil {
			return err
		}
	}
	if statusStr == "" || statusStr == "unsubmitted" {
		unsubmittedInvoices, err = fetchUnsubmittedInvoices(submittedInvoices)
		if err != nil {
			return err
		}
	}

	// Combine and sort the invoices.
	allInvoices := append(unsubmittedInvoices, submittedInvoices...)
	sort.Sort(sortableInvoices(allInvoices))

	if config.JSONOutput {
		output := invoices{
			Invoices: allInvoices,
		}
		bytes, err := json.Marshal(output)
		if err != nil {
			return err
		}
		Ctx.LastCommandOutput = string(bytes)
		return nil
	}

	fmt.Printf("Invoices: ")
	if len(allInvoices) == 0 {
		fmt.Printf("none\n")
	} else {
		for _, v := range allInvoices {
			fmt.Println()

			date := time.Date(int(v.Year), time.Month(v.Month),
				1, 0, 0, 0, 0, time.UTC)
			fmt.Printf("  %v • ", date.Format("January 2006"))
			if v.Token != nil {
				fmt.Printf("%v\n", *v.Token)
			} else {
				fmt.Printf("Draft\n")
			}

			if v.Timestamp != nil {
				fmt.Printf("    Submitted: %v\n",
					time.Unix(*v.Timestamp, 0).String())
			}

			fmt.Printf("       Status: %v\n", v.Status)
		}
	}

	return nil
}
