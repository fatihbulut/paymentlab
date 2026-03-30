package scheme

// ResponseCode represents an ISO8583 response code with scheme-specific descriptions
type ResponseCode struct {
	Code        string
	Description string
	Mastercard  string
	Visa        string
	Troy        string
}

// Common response codes with scheme-specific descriptions
var ResponseCodes = map[string]ResponseCode{
	"00": {Code: "00", Description: "Approved", Mastercard: "Approved", Visa: "Approved", Troy: "Onaylandı"},
	"01": {Code: "01", Description: "Refer to card issuer", Mastercard: "Refer to card issuer", Visa: "Refer to card issuer", Troy: "Kart sahibine başvurun"},
	"03": {Code: "03", Description: "Invalid merchant", Mastercard: "Invalid merchant", Visa: "Invalid merchant", Troy: "Geçersiz üye işyeri"},
	"04": {Code: "04", Description: "Capture card", Mastercard: "Pick up card", Visa: "Pick up card", Troy: "Kartı alın"},
	"05": {Code: "05", Description: "Do not honor", Mastercard: "Do not honor", Visa: "Do not honor", Troy: "İşlem onaylanmadı"},
	"06": {Code: "06", Description: "Error", Mastercard: "Error", Visa: "Error", Troy: "Hata"},
	"07": {Code: "07", Description: "Pick up card, special condition", Mastercard: "Pick up card, special condition", Visa: "Pick up card, special condition", Troy: "Kartı alın, özel durum"},
	"12": {Code: "12", Description: "Invalid transaction", Mastercard: "Invalid transaction", Visa: "Invalid transaction", Troy: "Geçersiz işlem"},
	"13": {Code: "13", Description: "Invalid amount", Mastercard: "Invalid amount", Visa: "Invalid amount", Troy: "Geçersiz tutar"},
	"14": {Code: "14", Description: "Invalid card number", Mastercard: "Invalid card number", Visa: "Invalid card number", Troy: "Geçersiz kart numarası"},
	"15": {Code: "15", Description: "No such issuer", Mastercard: "No such issuer", Visa: "No such issuer", Troy: "Böyle bir banka yok"},
	"19": {Code: "19", Description: "Re-enter transaction", Mastercard: "Re-enter transaction", Visa: "Re-enter transaction", Troy: "İşlemi tekrar girin"},
	"30": {Code: "30", Description: "Format error", Mastercard: "Format error", Visa: "Format error", Troy: "Format hatası"},
	"41": {Code: "41", Description: "Lost card", Mastercard: "Lost card, pick up", Visa: "Lost card", Troy: "Kayıp kart"},
	"43": {Code: "43", Description: "Stolen card", Mastercard: "Stolen card, pick up", Visa: "Stolen card", Troy: "Çalıntı kart"},
	"51": {Code: "51", Description: "Insufficient funds", Mastercard: "Insufficient funds", Visa: "Insufficient funds", Troy: "Yetersiz bakiye"},
	"54": {Code: "54", Description: "Expired card", Mastercard: "Expired card", Visa: "Expired card", Troy: "Kartın süresi dolmuş"},
	"55": {Code: "55", Description: "Incorrect PIN", Mastercard: "Incorrect PIN", Visa: "Incorrect PIN", Troy: "Hatalı PIN"},
	"56": {Code: "56", Description: "No card record", Mastercard: "No card record", Visa: "No card record", Troy: "Kart kaydı bulunamadı"},
	"57": {Code: "57", Description: "Transaction not permitted to cardholder", Mastercard: "Transaction not permitted to cardholder", Visa: "Transaction not permitted to cardholder", Troy: "İşlem kart sahibine izin verilmiyor"},
	"58": {Code: "58", Description: "Transaction not permitted to terminal", Mastercard: "Transaction not permitted to terminal", Visa: "Transaction not permitted to terminal", Troy: "İşlem terminale izin verilmiyor"},
	"61": {Code: "61", Description: "Exceeds withdrawal amount limit", Mastercard: "Exceeds withdrawal amount limit", Visa: "Exceeds withdrawal amount limit", Troy: "Çekim limiti aşıldı"},
	"62": {Code: "62", Description: "Restricted card", Mastercard: "Restricted card", Visa: "Restricted card", Troy: "Kısıtlı kart"},
	"63": {Code: "63", Description: "Security violation", Mastercard: "Security violation", Visa: "Security violation", Troy: "Güvenlik ihlali"},
	"65": {Code: "65", Description: "Exceeds withdrawal frequency limit", Mastercard: "Exceeds withdrawal frequency limit", Visa: "Exceeds withdrawal frequency limit", Troy: "Çekim sıklığı limiti aşıldı"},
	"75": {Code: "75", Description: "Allowable number of PIN tries exceeded", Mastercard: "PIN tries exceeded", Visa: "PIN tries exceeded", Troy: "PIN deneme sayısı aşıldı"},
	"76": {Code: "76", Description: "Invalid/nonexistent account", Mastercard: "Invalid/nonexistent account", Visa: "Invalid/nonexistent account", Troy: "Geçersiz hesap"},
	"78": {Code: "78", Description: "Blocked, first used", Mastercard: "Blocked, first used", Visa: "Blocked, first used", Troy: "Bloke, ilk kullanım"},
	"82": {Code: "82", Description: "Negative CAM, dCVV, iCVV, or CVV results", Mastercard: "Negative CAM, dCVV, iCVV, or CVV results", Visa: "Negative CAM, dCVV, iCVV, or CVV results", Troy: "CVV doğrulama hatası"},
	"85": {Code: "85", Description: "No reason to decline", Mastercard: "No reason to decline", Visa: "No reason to decline", Troy: "Reddetme nedeni yok"},
	"91": {Code: "91", Description: "Issuer or switch inoperative", Mastercard: "Issuer or switch inoperative", Visa: "Issuer unavailable", Troy: "Banka erişilemez"},
	"94": {Code: "94", Description: "Duplicate transmission", Mastercard: "Duplicate transmission", Visa: "Duplicate transmission", Troy: "Tekrarlanan işlem"},
	"96": {Code: "96", Description: "System malfunction", Mastercard: "System malfunction", Visa: "System error", Troy: "Sistem hatası"},
}

// GetResponseDescription returns the description for a response code
func GetResponseDescription(code string, scheme CardScheme) string {
	rc, ok := ResponseCodes[code]
	if !ok {
		return "Unknown response code"
	}

	switch scheme {
	case SchemeMastercard:
		return rc.Mastercard
	case SchemeVisa:
		return rc.Visa
	case SchemeTroy:
		return rc.Troy
	default:
		return rc.Description
	}
}

// IsApproved checks if the response code indicates approval
func IsApproved(code string) bool {
	return code == "00" || code == "10" || code == "11"
}

// IsDeclined checks if the response code indicates decline
func IsDeclined(code string) bool {
	declineCodes := map[string]bool{
		"05": true, "14": true, "41": true, "43": true,
		"51": true, "54": true, "55": true, "57": true,
		"61": true, "62": true, "65": true, "75": true,
	}
	return declineCodes[code]
}

// IsSystemError checks if the response code indicates a system error
func IsSystemError(code string) bool {
	return code == "91" || code == "96"
}
