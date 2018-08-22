package commands

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type bittrexGetTicksResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Result  []bittrexGetTicksResult `json:"result"`
}

type bittrexGetTicksResult struct {
	BaseVolume float64 `json:"bv"`
	Close      float64 `json:"c"`
	High       float64 `json:"h"`
	Low        float64 `json:"l"`
	Open       float64 `json:"o"`
	Timestamp  string  `json:"t"`
	Volume     float64 `json:"v"`
}

type bittrexGetTicksIteratorFunc func(bittrexGetTicksResult) error

const (
	bittrexTickTimestampFormat = "2006-01-02T15:04:05"
	bittrexGetTicksURL         = "https://bittrex.com/Api/v2.0/pub/market/GetTicks"
)

var (
	bittrexTickIntervals = []string{
		"thirtyMin",
		"hour",
		"day",
	}
)

func (cmd *DCRUSDCmd) iterateBittrexGetTicksResult(
	ticks []bittrexGetTicksResult,
	startOfMonth, endOfMonth time.Time,
	iterFunc bittrexGetTicksIteratorFunc,
) error {
	for _, tick := range ticks {
		tickTime, err := time.Parse(bittrexTickTimestampFormat, tick.Timestamp)
		if err != nil {
			return err
		}

		if tickTime.After(endOfMonth) {
			break
		}
		if tickTime.Before(startOfMonth) {
			continue
		}

		if err = iterFunc(tick); err != nil {
			return err
		}
	}

	return nil
}

func (cmd *DCRUSDCmd) getBittrexWeightedAverage(ticker string) (float64, string, error) {
	for idx, tickInterval := range bittrexTickIntervals {
		startOfMonthUnixMs := cmd.StartOfMonth.Unix() * 1000

		v := url.Values{}
		v.Set("marketName", ticker)
		v.Set("tickInterval", tickInterval) // Change to try thirtyMin first, then hour
		v.Set("_", strconv.FormatInt(startOfMonthUnixMs, 10))
		url := bittrexGetTicksURL + "?" + v.Encode()

		var resp bittrexGetTicksResponse
		err := cmd.makeRequest(url, &resp)
		if err != nil {
			return 0, "", err
		}

		if len(resp.Result) == 0 {
			return 0, "", fmt.Errorf("no results returned")
		}

		firstTickTime, err := time.Parse(bittrexTickTimestampFormat,
			resp.Result[0].Timestamp)
		if err != nil {
			return 0, "", err
		}
		if firstTickTime.After(cmd.StartOfMonth) {
			if idx < len(bittrexTickIntervals)-1 {
				// Try the next tick interval.
				continue
			}

			return 0, "", fmt.Errorf("data returned is not old enough; earliest date"+
				" is: %v", firstTickTime)
		}

		lastTickTime, err := time.Parse(bittrexTickTimestampFormat,
			resp.Result[len(resp.Result)-1].Timestamp)
		if err != nil {
			return 0, "", err
		}
		if lastTickTime.Before(cmd.EndOfMonth) {
			return 0, "", fmt.Errorf("data returned is not old enough; latest date"+
				" is: %v", lastTickTime)
		}

		var totalVolume float64
		err = cmd.iterateBittrexGetTicksResult(resp.Result, cmd.StartOfMonth,
			cmd.EndOfMonth,
			func(tick bittrexGetTicksResult) error {
				totalVolume += tick.Volume
				return nil
			})
		if err != nil {
			return 0, "", err
		}

		var weightedAverage float64
		err = cmd.iterateBittrexGetTicksResult(resp.Result, cmd.StartOfMonth,
			cmd.EndOfMonth,
			func(tick bittrexGetTicksResult) error {
				/*
					fmt.Printf("%v\n", tickTime)
					fmt.Printf("  %v\n", tick.Close)
					fmt.Printf("  %v\n", tick.Volume/totalVolume)
				*/
				weightedAverage += tick.Close * (tick.Volume / totalVolume)
				return nil
			})

		return weightedAverage, tickInterval, err
	}

	return 0, "", fmt.Errorf("invalid code path")
}

func (cmd *DCRUSDCmd) getBittrexValue() (float64, error) {
	btcDCRVal, btcDCRTickInterval, err := cmd.getBittrexWeightedAverage("BTC-DCR")
	if err != nil {
		return 0, err
	}

	usdBTCVal, usdBTCTickInterval, err := cmd.getBittrexWeightedAverage("USD-BTC")
	if err != nil {
		return 0, err
	}

	if config.Verbose {
		fmt.Printf("Calculated Bittrex DCR-USD value using DCR-BTC (%v) and"+
			" BTC-USD (%v)\n", btcDCRTickInterval, usdBTCTickInterval)
	}
	return btcDCRVal * usdBTCVal, nil
}
