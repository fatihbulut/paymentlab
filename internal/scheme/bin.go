package scheme

import "strings"

// CardScheme represents a payment card network
type CardScheme string

const (
	SchemeMastercard CardScheme = "MASTERCARD"
	SchemeVisa       CardScheme = "VISA"
	SchemeTroy       CardScheme = "TROY"
	SchemeUnknown    CardScheme = "UNKNOWN"
)

// BINRange represents a Bank Identification Number range
type BINRange struct {
	Prefix    string
	MinLength int
	MaxLength int
	Scheme    CardScheme
}

// binRanges contains known BIN ranges for each scheme
// Order matters: more specific prefixes should come first
var binRanges = []BINRange{
	// Troy (Turkey) - 9792xx
	{Prefix: "9792", MinLength: 16, MaxLength: 16, Scheme: SchemeTroy},

	// Mastercard - 51-55, 2221-2720
	{Prefix: "51", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "52", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "53", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "54", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "55", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2221", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2222", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2223", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2224", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2225", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2226", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2227", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2228", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2229", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "223", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "224", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "225", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "226", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "227", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "228", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "229", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "23", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "24", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "25", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "26", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "270", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "271", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},
	{Prefix: "2720", MinLength: 16, MaxLength: 16, Scheme: SchemeMastercard},

	// Visa - 4xxx
	{Prefix: "4", MinLength: 13, MaxLength: 19, Scheme: SchemeVisa},
}

// DetectScheme identifies the card scheme from a PAN
func DetectScheme(pan string) CardScheme {
	pan = strings.TrimSpace(pan)
	if len(pan) < 13 {
		return SchemeUnknown
	}

	for _, br := range binRanges {
		if strings.HasPrefix(pan, br.Prefix) {
			if len(pan) >= br.MinLength && len(pan) <= br.MaxLength {
				return br.Scheme
			}
		}
	}

	return SchemeUnknown
}

// GetBIN extracts the first 6 digits (BIN) from a PAN
func GetBIN(pan string) string {
	pan = strings.TrimSpace(pan)
	if len(pan) < 6 {
		return ""
	}
	return pan[:6]
}

// GetBIN8 extracts the first 8 digits (extended BIN) from a PAN
func GetBIN8(pan string) string {
	pan = strings.TrimSpace(pan)
	if len(pan) < 8 {
		return ""
	}
	return pan[:8]
}

// IsValidPANLength checks if PAN length is valid for the detected scheme
func IsValidPANLength(pan string) bool {
	scheme := DetectScheme(pan)
	length := len(strings.TrimSpace(pan))

	switch scheme {
	case SchemeMastercard, SchemeTroy:
		return length == 16
	case SchemeVisa:
		return length >= 13 && length <= 19
	default:
		return length >= 13 && length <= 19
	}
}
