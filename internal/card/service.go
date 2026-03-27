package card

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"iso-parser-service/internal/store"
)

type Service struct {
	store store.Store
}

func NewService(s store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) CreateCard(ctx context.Context, req CreateCardRequest) (*CardResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	pan := strings.ReplaceAll(strings.TrimSpace(req.PAN), " ", "")
	if pan == "" {
		return nil, fmt.Errorf("pan is required")
	}
	if !isDigits(pan) {
		return nil, fmt.Errorf("pan must be numeric")
	}
	if !luhnValid(pan) {
		return nil, fmt.Errorf("pan failed luhn check")
	}

	exp := strings.TrimSpace(req.ExpiryDate)
	if len(exp) != 4 || !isDigits(exp) {
		return nil, fmt.Errorf("expiry_date must be YYMM")
	}

	scheme := strings.TrimSpace(req.Scheme)
	if scheme == "" {
		scheme = "GENERIC"
	}

	cur := strings.TrimSpace(req.CurrencyCode)
	if cur == "" {
		cur = "949"
	}

	var pinHash *string
	if req.Pin != nil && strings.TrimSpace(*req.Pin) != "" {
		h := sha256.Sum256([]byte(strings.TrimSpace(*req.Pin)))
		hexStr := hex.EncodeToString(h[:])
		pinHash = &hexStr
	}

	var cvvHash *string
	if req.CVV != nil && strings.TrimSpace(*req.CVV) != "" {
		h := sha256.Sum256([]byte(strings.TrimSpace(*req.CVV)))
		hexStr := hex.EncodeToString(h[:])
		cvvHash = &hexStr
	}

	c := &store.Card{
		PAN:              pan,
		ExpiryDate:       exp,
		Status:           store.CardStatus("ACTIVE"),
		Scheme:           scheme,
		CurrencyCode:     cur,
		CreditLimit:      req.CreditLimit,
		AvailableBalance: req.AvailableBalance,
		PinHash:          pinHash,
		CvvHash:          cvvHash,
	}

	created, err := s.store.Cards().CreateCard(ctx, c)
	if err != nil {
		return nil, err
	}

	return &CardResponse{
		ID:               created.ID,
		PAN:              created.PAN,
		PANMasked:        maskPAN(created.PAN),
		ExpiryDate:       created.ExpiryDate,
		Status:           string(created.Status),
		Scheme:           created.Scheme,
		CurrencyCode:     created.CurrencyCode,
		CreditLimit:      created.CreditLimit,
		AvailableBalance: created.AvailableBalance,
	}, nil
}

func (s *Service) ListCards(ctx context.Context, limit int, offset int) ([]CardResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	cards, err := s.store.Cards().ListCards(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	out := make([]CardResponse, 0, len(cards))
	for _, c := range cards {
		out = append(out, CardResponse{
			ID:               c.ID,
			PAN:              c.PAN,
			PANMasked:        maskPAN(c.PAN),
			ExpiryDate:       c.ExpiryDate,
			Status:           string(c.Status),
			Scheme:           c.Scheme,
			CurrencyCode:     c.CurrencyCode,
			CreditLimit:      c.CreditLimit,
			AvailableBalance: c.AvailableBalance,
		})
	}
	return out, nil
}

func (s *Service) UpdateCard(ctx context.Context, id string, req UpdateCardRequest) (*CardResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	existing, err := s.store.Cards().GetCardByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("card not found")
	}

	exp := strings.TrimSpace(req.ExpiryDate)
	if exp == "" {
		exp = existing.ExpiryDate
	} else if len(exp) != 4 || !isDigits(exp) {
		return nil, fmt.Errorf("expiry_date must be YYMM")
	}

	scheme := strings.TrimSpace(req.Scheme)
	if scheme == "" {
		scheme = existing.Scheme
	}

	cur := strings.TrimSpace(req.CurrencyCode)
	if cur == "" {
		cur = existing.CurrencyCode
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = string(existing.Status)
	}

	existing.ExpiryDate = exp
	existing.Scheme = scheme
	existing.CurrencyCode = cur
	if req.CreditLimit != nil {
		existing.CreditLimit = *req.CreditLimit
	}
	if req.AvailableBalance != nil {
		existing.AvailableBalance = *req.AvailableBalance
	}
	existing.Status = store.CardStatus(status)

	updated, err := s.store.Cards().UpdateCard(ctx, existing)
	if err != nil {
		return nil, err
	}

	return &CardResponse{
		ID:               updated.ID,
		PAN:              updated.PAN,
		PANMasked:        maskPAN(updated.PAN),
		ExpiryDate:       updated.ExpiryDate,
		Status:           string(updated.Status),
		Scheme:           updated.Scheme,
		CurrencyCode:     updated.CurrencyCode,
		CreditLimit:      updated.CreditLimit,
		AvailableBalance: updated.AvailableBalance,
	}, nil
}

func (s *Service) DeleteCard(ctx context.Context, id string) error {
	if s.store == nil {
		return fmt.Errorf("store is nil")
	}
	return s.store.Cards().SoftDeleteCard(ctx, id)
}

func isDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func luhnValid(pan string) bool {
	var sum int
	alt := false
	for i := len(pan) - 1; i >= 0; i-- {
		d := int(pan[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

func maskPAN(pan string) string {
	if len(pan) <= 6 {
		return strings.Repeat("*", len(pan))
	}
	if len(pan) <= 10 {
		return pan[:4] + strings.Repeat("*", len(pan)-4)
	}
	return pan[:4] + strings.Repeat("*", len(pan)-8) + pan[len(pan)-4:]
}

func (s *Service) TopUp(ctx context.Context, id string, amount int64) (*CardResponse, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if err := s.store.Cards().CreditBalance(ctx, id, amount); err != nil {
		return nil, err
	}
	card, err := s.store.Cards().GetCardByID(ctx, id)
	if err != nil || card == nil {
		return nil, fmt.Errorf("card not found after top-up")
	}
	return &CardResponse{
		ID:               card.ID,
		PAN:              card.PAN,
		PANMasked:        maskPAN(card.PAN),
		ExpiryDate:       card.ExpiryDate,
		Status:           string(card.Status),
		Scheme:           card.Scheme,
		CurrencyCode:     card.CurrencyCode,
		CreditLimit:      card.CreditLimit,
		AvailableBalance: card.AvailableBalance,
	}, nil
}
