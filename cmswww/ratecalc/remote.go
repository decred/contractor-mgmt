package ratecalc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/decred/politeia/util"
)

type UnmarshalToCandlesticksFunc func([]byte) ([]Candlestick, error)
type DCRBTCData [][]interface{}
type BTCUSDData struct {
	Result XXBTZUSD `json:"result"`
}
type XXBTZUSD struct {
	XXBTZUSD [][]interface{} `json:"XXBTZUSD"`
}

const (
	dcrBTCUrl = "https://api.binance.com/api/v1/klines?symbol=DCRBTC&interval=%v&startTime=%v&endTime=%v"
	btcUSDUrl = "https://api.kraken.com/0/public/OHLC?pair=XBTUSD&interval=%v&since=%v"
)

var (
	dcrBTCDataThrottle = time.Tick(time.Second * 5)
	btcUSDDataThrottle = time.Tick(time.Second * 5)

	dcrBTCIntervalStrs = map[time.Duration]string{
		(time.Minute * 1):  "1m",
		(time.Minute * 5):  "5m",
		(time.Minute * 15): "15m",
		(time.Minute * 30): "30m",
		(time.Hour * 1):    "1h",
		(time.Hour * 4):    "4h",
	}

	btcUSDIntervalStrs = map[time.Duration]string{
		(time.Minute * 1):  "1",
		(time.Minute * 5):  "5",
		(time.Minute * 15): "15",
		(time.Minute * 30): "30",
		(time.Hour * 1):    "60",
		(time.Hour * 4):    "240",
	}
)

func getStringAsFloat(candlestickArr []interface{}, idx int) (float64, error) {
	val, ok := candlestickArr[idx].(string)
	if !ok {
		return 0, fmt.Errorf("data value not recognized as string: %v %v %v",
			candlestickArr, idx, reflect.TypeOf(candlestickArr[idx]))
	}

	return strconv.ParseFloat(val, 64)
}

func getFloat(candlestickArr []interface{}, idx int) (float64, error) {
	val, ok := candlestickArr[idx].(float64)
	if !ok {
		return 0, fmt.Errorf("data value not recognized as float64: %v %v %v",
			candlestickArr, idx, reflect.TypeOf(candlestickArr[idx]))
	}

	return val, nil
}

func (c *Calculator) makeRequest(url string) ([]byte, error) {
	log.Tracef("GET %v\n", url)
	req, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()

	responseBody := util.ConvertBodyToByteArray(req.Body, false)

	if req.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v: %v", req.Status, string(responseBody))
	}

	return responseBody, nil
}

func (c *Calculator) getCandlestickData(url string, fn UnmarshalToCandlesticksFunc) ([]Candlestick, error) {
	response, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	return fn(response)
}

func (c *Calculator) getDCRBTCData(
	currIntervalTime time.Time,
	interval time.Duration,
) ([]Candlestick, error) {
	rangeStartTime := currIntervalTime.Unix() * 1000
	rangeEndTime := currIntervalTime.Add(interval*20).Unix() * 1000
	url := fmt.Sprintf(dcrBTCUrl, dcrBTCIntervalStrs[interval],
		rangeStartTime, rangeEndTime)

	<-dcrBTCDataThrottle
	return c.getCandlestickData(url,
		func(response []byte) ([]Candlestick, error) {
			var data DCRBTCData
			err := json.Unmarshal(response, &data)
			if err != nil {
				return nil, err
			}

			if len(data) == 0 {
				// No data was returned.
				return []Candlestick{}, nil
			}

			candlesticks := make([]Candlestick, 0, len(data))
			for _, candlestickArr := range data {
				timestamp, err := getFloat(candlestickArr, 0)
				if err != nil {
					return nil, err
				}

				open, err := getStringAsFloat(candlestickArr, 1)
				if err != nil {
					return nil, err
				}

				high, err := getStringAsFloat(candlestickArr, 2)
				if err != nil {
					return nil, err
				}

				low, err := getStringAsFloat(candlestickArr, 3)
				if err != nil {
					return nil, err
				}

				close, err := getStringAsFloat(candlestickArr, 4)
				if err != nil {
					return nil, err
				}

				volume, err := getStringAsFloat(candlestickArr, 5)
				if err != nil {
					return nil, err
				}

				c := Candlestick{
					Timestamp:   int64(timestamp) / 1000,
					Granularity: int64(interval.Minutes()),
					Open:        open,
					Close:       close,
					High:        high,
					Low:         low,
					Volume:      volume,
				}

				candlesticks = append(candlesticks, c)
			}

			return candlesticks, nil
		},
	)
}

func (c *Calculator) getBTCUSDData(
	currIntervalTime time.Time,
	interval time.Duration,
) ([]Candlestick, error) {
	// Kraken's range start is exclusive, so subtract by 1 interval.
	rangeStartTime := currIntervalTime.Add(-1 * interval).Unix()
	url := fmt.Sprintf(btcUSDUrl, btcUSDIntervalStrs[interval], rangeStartTime)

	<-btcUSDDataThrottle
	return c.getCandlestickData(url,
		func(response []byte) ([]Candlestick, error) {
			var data BTCUSDData
			err := json.Unmarshal(response, &data)
			if err != nil {
				return nil, err
			}

			if len(data.Result.XXBTZUSD) == 0 {
				// No data was returned.
				return []Candlestick{}, nil
			}

			candlesticks := make([]Candlestick, 0, len(data.Result.XXBTZUSD))
			for _, candlestickArr := range data.Result.XXBTZUSD {
				timestamp, err := getFloat(candlestickArr, 0)
				if err != nil {
					return nil, err
				}

				open, err := getStringAsFloat(candlestickArr, 1)
				if err != nil {
					return nil, err
				}

				high, err := getStringAsFloat(candlestickArr, 2)
				if err != nil {
					return nil, err
				}

				low, err := getStringAsFloat(candlestickArr, 3)
				if err != nil {
					return nil, err
				}

				close, err := getStringAsFloat(candlestickArr, 4)
				if err != nil {
					return nil, err
				}

				volume, err := getStringAsFloat(candlestickArr, 6)
				if err != nil {
					return nil, err
				}

				c := Candlestick{
					Timestamp:   int64(timestamp),
					Granularity: int64(interval.Minutes()),
					Open:        open,
					Close:       close,
					High:        high,
					Low:         low,
					Volume:      volume,
				}

				candlesticks = append(candlesticks, c)
			}

			return candlesticks, nil
		},
	)
}
