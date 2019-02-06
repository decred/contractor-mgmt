package commands

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type LogWorkCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" required:"true"`
}

var (
	file   *os.File
	policy *v1.PolicyReply
	month  uint16
	year   uint16
)

func writeHeader() error {
	_, err := fmt.Fprintf(file, "%v %v\n", string(policy.Invoice.CommentChar),
		config.GetInvoiceMonthStr(month, year))
	if err != nil {
		return err
	}

	invoiceFieldNames := make([]string, 0, len(policy.Invoice.Fields))
	for _, v := range policy.Invoice.Fields {
		invoiceFieldNames = append(invoiceFieldNames, v.Name)
	}
	_, err = fmt.Fprintf(file, "%v %v\n", string(policy.Invoice.CommentChar),
		strings.Join(invoiceFieldNames, ", "))
	return err
}

func promptForFieldValues() ([]string, error) {
	reader := bufio.NewReader(os.Stdin)

	invoiceFieldValues := make([]string, 0, len(policy.Invoice.Fields))
	idx := 0
	for idx < len(policy.Invoice.Fields) {
		field := policy.Invoice.Fields[idx]

		if !config.JSONOutput {
			var optionalStr string
			if !field.Required {
				optionalStr = " (optional)"
			}
			log.Printf("%v%v: ", field.Name, optionalStr)
		}
		valueStr, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		valueStr = strings.TrimSpace(valueStr)

		if field.Required && len(valueStr) == 0 {
			if config.JSONOutput {
				return nil, fmt.Errorf("This field is required")
			}
			fmt.Println("This field is required")
			continue
		}

		if field.Type == v1.InvoiceFieldTypeUint {
			value, err := strconv.ParseUint(valueStr, 10, 64)
			if err != nil || value == 0 {
				if config.JSONOutput {
					return nil, fmt.Errorf("This field must be a positive number")
				}
				fmt.Println("This field must be a positive number")
				continue
			}
		}

		invoiceFieldValues = append(invoiceFieldValues, valueStr)
		idx++
	}

	return invoiceFieldValues, nil
}

func writeValues(values []string) error {
	csvWriter := csv.NewWriter(file)
	csvWriter.Comma = policy.Invoice.FieldDelimiterChar
	csvWriter.UseCRLF = false

	err := csvWriter.Write(values)
	if err != nil {
		return err
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func (cmd *LogWorkCmd) Execute(args []string) error {
	err := InitialVersionRequest()
	if err != nil {
		return err
	}

	id := config.LoggedInUserIdentity
	if id == nil {
		return ErrNotLoggedIn
	}

	month, err = ParseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}
	year = cmd.Args.Year

	policy, err = fetchPolicy()
	if err != nil {
		return err
	}

	invoiceFile, err := config.GetInvoiceFilename(month, year)
	if err != nil {
		return err
	}
	invoiceAlreadyExists := config.FileExists(invoiceFile)

	file, err = os.OpenFile(invoiceFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	defer file.Close()

	if !invoiceAlreadyExists {
		err = writeHeader()
		if err != nil {
			return err
		}
	}

	invoiceValues, err := promptForFieldValues()
	if err != nil {
		return err
	}

	err = writeValues(invoiceValues)
	if err != nil {
		return err
	}

	if !config.JSONOutput {
		fmt.Println("Work logged successfully.")
	}
	return nil
}
