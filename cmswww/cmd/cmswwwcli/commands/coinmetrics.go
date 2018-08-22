package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

type coinmetricsGetAssetDataResponse struct {
	Result []coinmetricsGetAssetDataResult `json:"result"`
}

type coinmetricsGetAssetDataResult struct {
	Timestamp int64
	Data      float64
}

const (
	coinmetricsURL = "https://coinmetrics.io/api/v1/get_asset_data_for_time_range/dcr/%v/%v/%v"
)

func (cmr *coinmetricsGetAssetDataResult) UnmarshalJSON(data []byte) error {
	var v []interface{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return fmt.Errorf("error while decoding: %v", err)
	}

	cmr.Timestamp = int64(v[0].(float64))
	cmr.Data = v[1].(float64)
	return nil
}

func (cmd *DCRUSDCmd) getCoinmetricsData(
	dataType string,
	startOfMonth, endOfMonth time.Time,
) ([]float64, error) {

	url := fmt.Sprintf(coinmetricsURL, dataType, startOfMonth.Unix(),
		endOfMonth.Unix())

	var resp coinmetricsGetAssetDataResponse
	err := cmd.makeRequest(url, &resp)
	if err != nil {
		return nil, err
	}

	if len(resp.Result) == 0 {
		return nil, fmt.Errorf("no results returned")
	}

	data := make([]float64, 0, len(resp.Result))
	for _, v := range resp.Result {
		data = append(data, v.Data)
	}
	return data, nil
}

func (cmd *DCRUSDCmd) getCoinmetricsValue() (float64, error) {
	startOfMonth := time.Date(int(cmd.Year), time.Month(cmd.Month), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-1 * time.Nanosecond)

	prices, err := cmd.getCoinmetricsData("price(usd)", startOfMonth,
		endOfMonth)
	if err != nil {
		return 0, err
	}

	exchangeVolumes, err := cmd.getCoinmetricsData("exchangevolume(usd)",
		startOfMonth, endOfMonth)
	if err != nil {
		return 0, err
	}

	if len(prices) != len(exchangeVolumes) {
		return 0, fmt.Errorf("length of data returned is inconsistent: %v %v",
			len(prices), len(exchangeVolumes))
	}

	var totalVolume float64
	for _, volume := range exchangeVolumes {
		totalVolume += volume
	}

	var weightedAverage float64
	for idx, price := range prices {
		weightedAverage += price * (exchangeVolumes[idx] / totalVolume)
	}

	if config.Verbose {
		fmt.Printf("Calculated Coinmetrics DCR-USD value with %v data points\n",
			len(prices))
	}
	return weightedAverage, nil
}
