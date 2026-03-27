package auth

import (
	"fmt"
	"time"

	"iso-parser-service/internal/store"
)

func ValidateCardBasic(c *store.Card) (respCode string, declineReason string) {
	if c == nil {
		return RespInvalidCardNumber, "card not found"
	}

	if c.Status != store.CardStatus("ACTIVE") {
		return RespBlockedCard, "card is not active"
	}

	if !expiryValidYYMM(c.ExpiryDate, time.Now()) {
		return RespExpiredCard, "card expired"
	}

	return "", ""
}

func expiryValidYYMM(expYYMM string, now time.Time) bool {
	if len(expYYMM) != 4 {
		return false
	}
	// YYMM
	yy := expYYMM[0:2]
	mm := expYYMM[2:4]
	y, err := atoi2(yy)
	if err != nil {
		return false
	}
	m, err := atoi2(mm)
	if err != nil {
		return false
	}
	if m < 1 || m > 12 {
		return false
	}

	year := 2000 + y
	// Expiry usually valid through end of month
	firstOfNext := time.Date(year, time.Month(m)+1, 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := firstOfNext.Add(-time.Nanosecond)

	return now.UTC().Before(endOfMonth)
}

func atoi2(s string) (int, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("invalid len")
	}
	if s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return 0, fmt.Errorf("invalid digit")
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), nil
}
