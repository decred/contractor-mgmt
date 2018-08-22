package commands

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type DCRUSDCmd struct {
	Args struct {
		Month string `positional-arg-name:"month"`
		Year  uint16 `positional-arg-name:"year"`
	} `positional-args:"true" required:"true"`

	httpClient   *http.Client
	Month        uint16
	Year         uint16
	StartOfMonth time.Time
	EndOfMonth   time.Time
}

type DCRUSDSource struct {
	Name string
	Func DCRUSDValueSourceFunc
}

type DCRUSDValueSourceFunc func() (float64, error)

func (cmd *DCRUSDCmd) makeRequest(url string, resp interface{}) error {
	req, err := cmd.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer req.Body.Close()

	responseBody := util.ConvertBodyToByteArray(req.Body, false)

	if req.StatusCode != http.StatusOK {
		return fmt.Errorf("%v: %v", req.Status, string(responseBody))
	}

	return json.Unmarshal(responseBody, resp)
}

func (cmd *DCRUSDCmd) getMedianValue(values sort.Float64Slice) float64 {
	values.Sort()

	l := len(values)

	// Invalid input.
	if l == 0 {
		return math.NaN()
	}

	// Odd number of values, return the middle one.
	if l%2 == 1 {
		return values[l/2]
	}

	// Even number of values, average the middle two.
	return (values[(l/2)-1] + values[l/2]) / 2
}

func (cmd *DCRUSDCmd) Execute(args []string) error {
	var err error
	cmd.Month, err = ParseMonth(cmd.Args.Month)
	if err != nil {
		return err
	}
	cmd.Year = cmd.Args.Year

	cmd.StartOfMonth = time.Date(int(cmd.Year), time.Month(cmd.Month), 1, 0, 0, 0, 0, time.UTC)
	cmd.EndOfMonth = cmd.StartOfMonth.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)

	if config.Verbose {
		fmt.Printf("Start of month: %v\n", cmd.StartOfMonth.Unix())
		fmt.Printf("End of month: %v\n", cmd.EndOfMonth.Unix())
		fmt.Println()
	}

	cmd.httpClient = &http.Client{
		Transport: &http.Transport{
			IdleConnTimeout: 60 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	var values []float64
	sources := []DCRUSDSource{
		{
			Name: "Bittrex",
			Func: cmd.getBittrexValue,
		},
		{
			Name: "Coinmetrics",
			Func: cmd.getCoinmetricsValue,
		},
	}

	for _, source := range sources {
		value, err := source.Func()
		if err != nil {
			fmt.Printf("Error while fetching %v value: %v\n", source.Name, err)
		} else {
			values = append(values, value)
			fmt.Printf("%v: $%.2f\n", source.Name, value)
		}
		fmt.Println()
	}

	medianVal := cmd.getMedianValue(values)
	fmt.Printf("Median: $%.2f\n", medianVal)
	return nil
}
