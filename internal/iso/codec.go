package iso

import (
	"encoding/hex"
	"fmt"

	"github.com/moov-io/iso8583"
)


func ParseHexToMessage(hexStr string) (*ISOMessage, error) {
	if len(hexStr) == 0 {
		return nil, fmt.Errorf("empty hex string")
	}

	if len(hexStr)%2 != 0 {
		return nil, fmt.Errorf("invalid hex length: must be even")
	}

	rawBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	msg := iso8583.NewMessage(Spec)
	if err := msg.Unpack(rawBytes); err != nil {
		return nil, fmt.Errorf("unpack ISO8583 message: %w", err)
	}

	data := &ISOMessage{}
	if err := msg.Unmarshal(data); err != nil {
		return nil, fmt.Errorf("unmarshal ISOMessage: %w", err)
	}

	if mti, err := msg.GetMTI(); err == nil {
		data.MTI = mti
	}

	return data, nil
}

func PackMessageToHex(m *ISOMessage) (string, error) {
	if m == nil {
		return "", fmt.Errorf("message is nil")
	}

	msg := iso8583.NewMessage(Spec)
	msg.MTI(m.MTI)

	if err := msg.Marshal(m); err != nil {
		return "", fmt.Errorf("marshal ISOMessage: %w", err)
	}

	rawBytes, err := msg.Pack()
	if err != nil {
		return "", fmt.Errorf("pack ISO8583 message: %w", err)
	}

	encoded := hex.EncodeToString(rawBytes)
	return encoded, nil
}
