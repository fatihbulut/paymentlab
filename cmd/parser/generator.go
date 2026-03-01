package main

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// CreateAuthRequest: Mevcut spec'ini kullanarak 0100 mesajı üretir.
func CreateAuthRequest(spec *iso8583.MessageSpec, pan string, amount int64, stan string) ([]byte, error) {
	// 1. Yeni mesaj objesi
	message := iso8583.NewMessage(spec)

	// 2. MTI: Authorization Request
	message.MTI("0100")

	// 3. Temel Alanlar
	message.Field(2, pan)                          // Kart No
	message.Field(4, fmt.Sprintf("%012d", amount)) // Tutar (12 hane sabit)
	message.Field(11, stan)                        // Sistem Takip No
	message.Field(49, "949")                       // Para Birimi (TL)

	// 4. Paketleme (Header falan eklemiyoruz, saf ISO)
	packed, err := message.Pack()
	if err != nil {
		return nil, err
	}

	return packed, nil
}
