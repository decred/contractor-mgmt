package ratecalc

import (
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Calculator struct {
	sync.RWMutex // lock for file reading/writing

	dataDir    string
	httpClient *http.Client
}

type Candlestick struct {
	Timestamp   int64   // Timestamp of this interval
	Granularity int64   // Interval in minutes
	Open        float64 // Price at interval start
	Close       float64 // Price at interval end
	High        float64 // Highest price during this interval
	Low         float64 // Lowest price during this interval
	Volume      float64 // Volume for this interval
}

var (
	interval = time.Minute * 15

	ErrNonExistentLogFile = fmt.Errorf("non-existent log file")
	ErrNoRecordsFound     = fmt.Errorf("no records found")
)

func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func firstDayOfMonth(month time.Month, year int) time.Time {
	return time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
}

func New(dataDir string) *Calculator {
	calc := Calculator{
		dataDir: dataDir,
	}
	go calc.init()
	return &calc
}

func (c *Calculator) getLogFilename(month time.Month, year int) string {
	return filepath.Join(c.dataDir,
		fmt.Sprintf("rate-candlesticks-%v-%v.csv", year, month))
}

func (c *Calculator) init() {
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			IdleConnTimeout: 60 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	c.updateCandlesticks()
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.updateCandlesticks()
		}
	}()
}

func (c *Calculator) updateCandlesticks() {
	log.Infof("Updating candlesticks")

	t := time.Now()
	fifthDayOfMonth := firstDayOfMonth(t.Month(), t.Year()).AddDate(0, 0, 4)
	if t.Before(fifthDayOfMonth) {
		lastDayOfPrevMonth := firstDayOfMonth(t.Month(), t.Year()).Add(
			time.Nanosecond * -1)

		for {
			shouldContinue, err := c.updateCandlesticksForMonth(lastDayOfPrevMonth.Month(),
				lastDayOfPrevMonth.Year(), true)
			if err != nil {
				log.Error(err)
				break
			}
			if !shouldContinue {
				break
			}
		}
	}

	for {
		shouldContinue, err := c.updateCandlesticksForMonth(t.Month(), t.Year(), false)
		if err != nil {
			log.Error(err)
			break
		}
		if !shouldContinue {
			break
		}
	}
}

func (c *Calculator) getMostRecentIntervalFromLogFile(
	month time.Month,
	year int,
) (time.Time, error) {
	filename := c.getLogFilename(month, year)

	records, err := c.getRecords(filename)
	if err != nil {
		return time.Time{}, err
	}
	if records == nil || len(records) == 0 {
		return time.Time{}, nil
	}

	lastRecord := records[len(records)-1]
	lastInterval, err := convertStringArrayToCandlestick(lastRecord)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not convert record to Interval: %v", err)
	}

	return time.Unix(lastInterval.Timestamp, 0), nil
}

func (c *Calculator) shouldUpdateLogFile(
	intervalTime time.Time,
	month time.Month,
	year int,
) bool {
	firstDayOfNextMonth := firstDayOfMonth(month, year).AddDate(0, 1, 0)
	lastIntervalOfMonth := firstDayOfNextMonth.Add(interval * -1)

	// If the most recent time is the last interval of the month,
	// then we have nothing to do.
	if !intervalTime.Before(lastIntervalOfMonth) {
		return false
	}

	// If the most recent time is within the interval duration of the current
	// time, then we have nothing to do.
	if intervalTime.After(time.Now().Add(interval)) {
		return false
	}

	return true
}

func (c *Calculator) getDataIndices(
	currIntervalTime time.Time,
	dcrBTCData, btcUSDData []Candlestick,
) (time.Time, int, int) {
	// Ensure that the current interval time falls within the range
	// of DCR-BTC data.
	dcrBTCFirstIntervalTime := time.Unix(dcrBTCData[0].Timestamp, 0)
	if currIntervalTime.Before(dcrBTCFirstIntervalTime) {
		currIntervalTime = dcrBTCFirstIntervalTime
	}

	// Ensure that the current interval time falls within the range
	// of BTC-USD data.
	btcUSDFirstIntervalTime := time.Unix(btcUSDData[0].Timestamp, 0)
	if currIntervalTime.Before(btcUSDFirstIntervalTime) {
		currIntervalTime = btcUSDFirstIntervalTime
	}

	var (
		dcrBTCIdx int
		btcUSDIdx int
	)
	for dcrBTCIdx < len(dcrBTCData) && btcUSDIdx < len(btcUSDData) {
		dcrBTCCandlestick := dcrBTCData[dcrBTCIdx]
		btcUSDCandlestick := btcUSDData[btcUSDIdx]

		// Ensure the timestamps from the 2 datasets are aligned.
		if dcrBTCCandlestick.Timestamp < btcUSDCandlestick.Timestamp {
			dcrBTCIdx++
			continue
		} else if btcUSDCandlestick.Timestamp < dcrBTCCandlestick.Timestamp {
			btcUSDIdx++
			continue
		}

		// Find the indices for the current interval.
		if dcrBTCCandlestick.Timestamp < currIntervalTime.Unix() {
			dcrBTCIdx++
			btcUSDIdx++
			continue
		}

		return currIntervalTime, dcrBTCIdx, btcUSDIdx
	}

	// The current interval is past the range of data.
	return currIntervalTime, -1, -1
}

func (c *Calculator) fetchData(
	month time.Month,
	year int,
	currIntervalTime time.Time,
) ([][]Candlestick, time.Time, error) {
	var candlesticks [][]Candlestick

	if currIntervalTime.After(time.Now().Add(interval)) {
		return candlesticks, currIntervalTime, nil
	}

	dcrBTCData, err := c.getDCRBTCData(currIntervalTime, interval)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("could not fetch dcr-btc data: %v",
			err)
	}

	if len(dcrBTCData) == 0 {
		// No data was returned.
		return candlesticks, currIntervalTime, nil
	}

	btcUSDData, err := c.getBTCUSDData(currIntervalTime, interval)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("could not fetch btc-usd data: %v",
			err)
	}

	if len(btcUSDData) == 0 {
		// No data was returned.
		return candlesticks, currIntervalTime, nil
	}

	// Get the indices of the data which match the current interval time.
	var (
		dcrBTCIdx int
		btcUSDIdx int
	)
	currIntervalTime, dcrBTCIdx, btcUSDIdx = c.getDataIndices(
		currIntervalTime, dcrBTCData, btcUSDData)
	if dcrBTCIdx < 0 {
		return candlesticks, currIntervalTime, nil
	}

	for {
		if dcrBTCIdx >= len(dcrBTCData) || btcUSDIdx >= len(btcUSDData) {
			break
		}

		candlesticks = append(candlesticks, []Candlestick{
			dcrBTCData[dcrBTCIdx],
			btcUSDData[btcUSDIdx],
		})
		currIntervalTime = time.Unix(dcrBTCData[dcrBTCIdx].Timestamp, 0)

		dcrBTCIdx++
		btcUSDIdx++
	}

	return candlesticks, currIntervalTime, nil
}

func (c *Calculator) addDataToLogFile(
	month time.Month,
	year int,
	data [][]Candlestick,
) error {
	filename := c.getLogFilename(month, year)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE,
		0600)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, candlesticks := range data {
		dcrBTCCandlestick := convertCandlestickToStringArray(&candlesticks[0])
		btcUSDCandlestick := convertCandlestickToStringArray(&candlesticks[1])
		err := writer.Write(append(dcrBTCCandlestick, btcUSDCandlestick...))
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Calculator) updateCandlesticksForMonth(
	month time.Month,
	year int,
	ignoreIfNonexistent bool,
) (bool, error) {
	currIntervalTime, err := c.getMostRecentIntervalFromLogFile(month, year)
	if err != nil {
		return false, err
	}
	if currIntervalTime.IsZero() {
		if ignoreIfNonexistent {
			return false, nil
		}

		currIntervalTime = firstDayOfMonth(month, year)
	} else {
		// If the current interval time is non-zero, then it's the most recent
		// interval found in the log file, so start at the next interval.
		currIntervalTime = currIntervalTime.Add(interval)
	}

	if !c.shouldUpdateLogFile(currIntervalTime, month, year) {
		return false, nil
	}

	data, currIntervalTime, err := c.fetchData(month, year, currIntervalTime)
	if err != nil {
		return false, err
	}

	if len(data) == 0 {
		// No data was returned.
		return false, nil
	}

	err = c.addDataToLogFile(month, year, data)
	if err != nil {
		return false, err
	}

	shouldContinue := !currIntervalTime.After(time.Now().Add(interval))
	return shouldContinue, nil
}

func (c *Calculator) getRecords(filename string) ([][]string, error) {
	c.RLock()
	defer c.RUnlock()

	if !fileExists(filename) {
		return nil, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (c *Calculator) CalculateRateForMonth(
	month time.Month,
	year int,
) (float64, bool, error) {
	records, err := c.getRecords(c.getLogFilename(month, year))
	if err != nil {
		return 0, false, err
	}

	if len(records) == 0 {
		return 0, false, ErrNoRecordsFound
	}

	var isDataMissing bool
	var total float64
	for idx, record := range records {
		dcrBTCCandlestick, err := convertStringArrayToCandlestick(record[:7])
		if err != nil {
			return 0, false, err
		}

		if idx == 0 {
			currIntervalTime := time.Unix(dcrBTCCandlestick.Timestamp, 0)
			firstIntervalOfMonth := firstDayOfMonth(month, year).Add(interval)
			if currIntervalTime.After(firstIntervalOfMonth) {
				log.Debugf("data is missing because the first record's "+
					"timestamp (%v) is later than the first interval of the "+
					"month (%v)", currIntervalTime.Unix(),
					firstIntervalOfMonth.Unix())
				isDataMissing = true
			}
		} else if idx == len(records)-1 {
			currIntervalTime := time.Unix(dcrBTCCandlestick.Timestamp, 0)
			lastIntervalOfMonth := firstDayOfMonth(month, year).AddDate(
				0, 1, 0).Add(-1 * interval)
			if currIntervalTime.Before(lastIntervalOfMonth) {
				log.Debugf("data is missing because the last record's "+
					"timestamp (%v) is earlier than the last interval of the "+
					"month (%v)", currIntervalTime.Unix(),
					lastIntervalOfMonth.Unix())
				isDataMissing = true
			}
		}

		btcUSDCandlestick, err := convertStringArrayToCandlestick(record[7:])
		if err != nil {
			return 0, false, err
		}

		dcrBTCAvg := (dcrBTCCandlestick.Open + dcrBTCCandlestick.Close) / 2
		btcUSDAvg := (btcUSDCandlestick.Open + btcUSDCandlestick.Close) / 2
		total += dcrBTCAvg * btcUSDAvg
	}

	monthStartTime := firstDayOfMonth(month, year).Unix()
	monthEndTime := firstDayOfMonth(month, year).AddDate(0, 1, 0).Unix()
	numIntervalsInMonth := (monthStartTime - monthEndTime) /
		int64(interval/time.Second)

	if len(records) < int(numIntervalsInMonth) {
		log.Debugf("data is missing because the number of records (%v) is "+
			"less than the number of intervals in the month (%v)", len(records),
			numIntervalsInMonth)
		isDataMissing = true
	}

	return (total / float64(len(records))), isDataMissing, nil
}
