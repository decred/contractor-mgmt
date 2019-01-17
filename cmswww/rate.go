package main

import (
	"net/http"
	"time"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

func (c *cmswww) HandleRate(
	req interface{},
	user *database.User,
	w http.ResponseWriter,
	r *http.Request,
) (interface{}, error) {
	rate := req.(*v1.Rate)
	dcrUSDRate, isDataMissing, err := c.rateCalculator.CalculateRateForMonth(
		time.Month(rate.Month), int(rate.Year))
	if err != nil {
		return nil, err
	}

	return &v1.RateReply{
		DCRUSDRate:    dcrUSDRate,
		IsDataMissing: isDataMissing,
	}, nil
}
