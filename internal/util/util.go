package util

import (
	"fmt"
	"strings"
)

// MaskPAN masks the PAN (card number) for logging/tracing.
//
// Format:
// - 6 digits or less: fully masked (****)
// - 10 digits or less: first 4 + mask (4111****)
// - 10+ digits: first 4 + mask + last 4 (4111********1111)
//
// Example: "4111111111111111" → "4111********1111"
func MaskPAN(pan string) string {
	if len(pan) <= 6 {
		return strings.Repeat("*", len(pan))
	}
	if len(pan) <= 10 {
		return pan[:4] + strings.Repeat("*", len(pan)-4)
	}
	return pan[:4] + strings.Repeat("*", len(pan)-8) + pan[len(pan)-4:]
}

// ParseAmount12 parses a 12-digit ISO8583 amount field to int64.
// The amount is in cents (no decimal point).
//
// Example: "000000001234" → 1234 (12.34 currency units)
func ParseAmount12(s string) (int64, error) {
	if len(s) != 12 {
		return 0, fmt.Errorf("amount must be 12 digits")
	}
	var v int64
	for i := 0; i < 12; i++ {
		b := s[i]
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("amount must be digits")
		}
		v = v*10 + int64(b-'0')
	}
	return v, nil
}
