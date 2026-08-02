package scheme

import (
	"encoding/hex"
	"fmt"
)

// EMVTag represents a parsed EMV TLV tag
type EMVTag struct {
	Tag    string
	Length int
	Value  string
	Name   string
}

// Common EMV Tags
var EMVTagNames = map[string]string{
	"9F26": "Application Cryptogram",
	"9F27": "Cryptogram Information Data",
	"9F10": "Issuer Application Data",
	"9F37": "Unpredictable Number",
	"9F36": "Application Transaction Counter",
	"95":   "Terminal Verification Results",
	"9A":   "Transaction Date",
	"9C":   "Transaction Type",
	"9F02": "Amount Authorized",
	"9F03": "Amount Other",
	"9F1A": "Terminal Country Code",
	"5F2A": "Transaction Currency Code",
	"82":   "Application Interchange Profile",
	"9F34": "Cardholder Verification Method Results",
	"9F35": "Terminal Type",
	"9F1E": "Interface Device Serial Number",
	"84":   "Dedicated File Name",
	"9F09": "Application Version Number",
	"9F33": "Terminal Capabilities",
	"9F40": "Additional Terminal Capabilities",
	"9F06": "Application Identifier (AID)",
	"9F07": "Application Usage Control",
	"5F34": "PAN Sequence Number",
	"9F0D": "Issuer Action Code - Default",
	"9F0E": "Issuer Action Code - Denial",
	"9F0F": "Issuer Action Code - Online",
}

// ParseEMVData parses Field 55 ICC System Related Data (TLV format)
func ParseEMVData(hexData string) ([]EMVTag, error) {
	if hexData == "" {
		return nil, nil
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		return nil, fmt.Errorf("invalid hex data: %w", err)
	}

	var tags []EMVTag
	pos := 0

	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		// Parse tag (1 or 2 bytes)
		tag := ""
		if data[pos]&0x1F == 0x1F {
			// Two-byte tag
			if pos+1 >= len(data) {
				break
			}
			tag = fmt.Sprintf("%02X%02X", data[pos], data[pos+1])
			pos += 2
		} else {
			// One-byte tag
			tag = fmt.Sprintf("%02X", data[pos])
			pos++
		}

		if pos >= len(data) {
			break
		}

		// Parse length (1 or 2+ bytes)
		length := 0
		if data[pos]&0x80 == 0 {
			// Short form: length in single byte
			length = int(data[pos])
			pos++
		} else {
			// Long form: first byte indicates number of length bytes
			numLengthBytes := int(data[pos] & 0x7F)
			pos++
			if pos+numLengthBytes > len(data) {
				break
			}
			for i := 0; i < numLengthBytes; i++ {
				length = (length << 8) | int(data[pos])
				pos++
			}
		}

		if pos+length > len(data) {
			break
		}

		// Parse value
		value := hex.EncodeToString(data[pos : pos+length])
		pos += length

		name := EMVTagNames[tag]
		if name == "" {
			name = "Unknown Tag"
		}

		tags = append(tags, EMVTag{
			Tag:    tag,
			Length: length,
			Value:  value,
			Name:   name,
		})
	}

	return tags, nil
}

// GetEMVTagValue extracts a specific tag value from parsed EMV data
func GetEMVTagValue(tags []EMVTag, tagID string) string {
	for _, t := range tags {
		if t.Tag == tagID {
			return t.Value
		}
	}
	return ""
}

// BuildEMVData creates TLV encoded data from tags
func BuildEMVData(tags []EMVTag) (string, error) {
	var result []byte

	for _, t := range tags {
		// Add tag
		tagBytes, err := hex.DecodeString(t.Tag)
		if err != nil {
			return "", fmt.Errorf("invalid tag %s: %w", t.Tag, err)
		}
		result = append(result, tagBytes...)

		// Add length
		valueBytes, err := hex.DecodeString(t.Value)
		if err != nil {
			return "", fmt.Errorf("invalid value for tag %s: %w", t.Tag, err)
		}
		length := len(valueBytes)
		if length < 128 {
			result = append(result, byte(length))
		} else if length < 256 {
			result = append(result, 0x81, byte(length))
		} else {
			result = append(result, 0x82, byte(length>>8), byte(length&0xFF))
		}

		// Add value
		result = append(result, valueBytes...)
	}

	return hex.EncodeToString(result), nil
}

// ValidateEMVCryptogram performs basic validation on EMV cryptogram data
func ValidateEMVCryptogram(tags []EMVTag) error {
	// Check for required tags
	requiredTags := []string{"9F26", "9F27", "9F10", "9F37", "9F36"}

	for _, reqTag := range requiredTags {
		found := false
		for _, t := range tags {
			if t.Tag == reqTag {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing required EMV tag: %s (%s)", reqTag, EMVTagNames[reqTag])
		}
	}

	return nil
}
