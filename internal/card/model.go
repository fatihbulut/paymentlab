package card

type CreateCardRequest struct {
	PAN              string  `json:"pan"`
	ExpiryDate       string  `json:"expiry_date"`
	Scheme           string  `json:"scheme"`
	CurrencyCode     string  `json:"currency_code"`
	CreditLimit      int64   `json:"credit_limit"`
	AvailableBalance int64   `json:"available_balance"`
	Pin              *string `json:"pin"`
	CVV              *string `json:"cvv"`
}

type UpdateCardRequest struct {
	ExpiryDate       string `json:"expiry_date"`
	Scheme           string `json:"scheme"`
	CurrencyCode     string `json:"currency_code"`
	CreditLimit      *int64 `json:"credit_limit"`
	AvailableBalance *int64 `json:"available_balance"`
	Status           string `json:"status"`
}

type CardResponse struct {
	ID               string `json:"id"`
	PAN              string `json:"pan"`
	PANMasked        string `json:"pan_masked"`
	ExpiryDate       string `json:"expiry_date"`
	Status           string `json:"status"`
	Scheme           string `json:"scheme"`
	CurrencyCode     string `json:"currency_code"`
	CreditLimit      int64  `json:"credit_limit"`
	AvailableBalance int64  `json:"available_balance"`
}
