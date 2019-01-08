package ratecalc

import (
	"strconv"
)

func convertCandlestickToStringArray(candlestick *Candlestick) []string {
	return []string{
		strconv.FormatInt(candlestick.Timestamp, 10),
		strconv.FormatInt(candlestick.Granularity, 10),
		strconv.FormatFloat(candlestick.Open, 'f', 8, 64),
		strconv.FormatFloat(candlestick.Close, 'f', 8, 64),
		strconv.FormatFloat(candlestick.High, 'f', 8, 64),
		strconv.FormatFloat(candlestick.Low, 'f', 8, 64),
		strconv.FormatFloat(candlestick.Volume, 'f', 8, 64),
	}
}

func convertStringArrayToCandlestick(record []string) (*Candlestick, error) {
	var candlestick Candlestick

	var err error
	candlestick.Timestamp, err = strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return nil, err
	}

	candlestick.Granularity, err = strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return nil, err
	}

	candlestick.Open, err = strconv.ParseFloat(record[2], 64)
	if err != nil {
		return nil, err
	}

	candlestick.Close, err = strconv.ParseFloat(record[3], 64)
	if err != nil {
		return nil, err
	}

	candlestick.High, err = strconv.ParseFloat(record[4], 64)
	if err != nil {
		return nil, err
	}

	candlestick.Low, err = strconv.ParseFloat(record[5], 64)
	if err != nil {
		return nil, err
	}

	candlestick.Volume, err = strconv.ParseFloat(record[6], 64)
	if err != nil {
		return nil, err
	}

	return &candlestick, nil
}
