package commands

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/crypto/sha3"
)

var (
	monthNames = map[string]uint16{
		"january":   1,
		"february":  2,
		"march":     3,
		"april":     4,
		"may":       5,
		"june":      6,
		"july":      7,
		"august":    8,
		"september": 9,
		"october":   10,
		"november":  11,
		"december":  12,
	}

	ErrNotLoggedIn = fmt.Errorf("You must be logged in to perform this action.")
)

func ParseMonth(monthStr string) (uint16, error) {
	if monthStr == "" {
		return 0, nil
	}

	parsedMonth, err := strconv.ParseUint(monthStr, 10, 16)
	if err == nil {
		return uint16(parsedMonth), nil
	}

	monthStr = strings.ToLower(monthStr)
	month, ok := monthNames[monthStr]
	if ok {
		return month, nil
	}

	for monthName, monthVal := range monthNames {
		if strings.Index(monthName, monthStr) == 0 {
			return monthVal, nil
		}
	}

	return 0, fmt.Errorf("invalid month specified")
}

// Digest returns the hex encoded SHA3-256 of a string.
func DigestSHA3(s string) string {
	h := sha3.New256()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
