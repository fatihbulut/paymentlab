## iso-parser-service

ISO8583 mesajlarını parse/pack eden ve acquirer–issuer akışını simüle eden bir Go projesi.

### Mimari

- **`internal/iso`**: ISO8583 alan tanımları (`Spec`), `ISOMessage` modeli ve `ParseHexToMessage` / `PackMessageToHex` codec fonksiyonları.
- **`cmd/issuer` + `internal/issuer`**: TCP üzerinden ISO8583 isteği alan, iş kuralına göre yanıt üreten issuer servisi.
- **`cmd/acquirer` + `internal/acquirer`**: HTTP üzerinden istek alıp ISO8583 mesajını issuer’a TCP ile ileten ve yanıtı parse edip JSON dönen acquirer servisi.
- **`cmd/parser`**: Doğrudan hex ↔ JSON parse/pack için yardımcı HTTP servisi (debug / geliştirme amaçlı).

### Çalıştırma

Issuer’ı TCP üzerinde dinleyecek şekilde başlat:

```bash
go run ./cmd/issuer
```

Varsayılan adres: `localhost:5001` (env: `ISSUER_ADDR`).

Ardından acquirer’ı HTTP ile başlat:

```bash
go run ./cmd/acquirer
```

Varsayılan HTTP portu: `:8081` (env: `ACQUIRER_PORT`), issuer adresi: `localhost:5001` (env: `ISSUER_ADDR`).

### Örnek istek (acquirer üzerinden)

```bash
curl -X POST http://localhost:8081/v1/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "MTI": "0100",
    "PAN": "4242424242424242",
    "ProcCode": "000000",
    "AmountTrn": "000000001000",
    "LocalTime": "123456",
    "LocalDate": "0101",
    "STAN": "123456",
    "TerminalID": "TERM0001",
    "MerchantID": "MERC00000000001"
  }'
```

Yanıtta hem issuer’a giden/gelen raw ISO8583 hex içeriği hem de parse edilmiş `ISOMessage` döner:

- `request_hex`: issuer’a gönderilen ISO8583 hex
- `response_hex`: issuer’dan dönen ISO8583 hex
- `response`: parse edilmiş yanıt mesajı
