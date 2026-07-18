package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/scheme"
	"iso-parser-service/internal/store"
	"iso-parser-service/internal/util"
)

type Engine struct {
	store store.Store
}

type Decision struct {
	RespCode      string
	AuthCode      *string
	DeclineReason *string
	BalanceBefore *int64
	BalanceAfter  *int64
	DurationMs    float64
	Scheme        scheme.CardScheme
}

func NewEngine(s store.Store) *Engine {
	return &Engine{store: s}
}

func (e *Engine) Authorize(ctx context.Context, req *iso.ISOMessage) (*iso.ISOMessage, *Decision, error) {
	start := time.Now()
	if req == nil {
		return nil, nil, fmt.Errorf("request is nil")
	}
	if e.store == nil {
		return nil, nil, fmt.Errorf("store is nil")
	}

	decision := &Decision{}
	defer func() {
		decision.DurationMs = float64(time.Since(start).Seconds() * 1000)
	}()

	// Detect card scheme (Mastercard, Visa, Troy)
	decision.Scheme = scheme.DetectScheme(req.PAN)

	// Route reversals to dedicated handler
	if len(req.MTI) == 4 && req.MTI[:2] == "04" {
		return e.handleReversal(ctx, req, decision)
	}

	// Build base response
	resp := *req
	if len(resp.MTI) == 4 {
		resp.MTI = resp.MTI[:2] + "10"
	}

	amount, err := util.ParseAmount12(req.AmountTrn)
	if err != nil {
		decision.RespCode = RespDoNotHonor
		resp.RespCode = decision.RespCode
		reason := "invalid amount"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	// Single atomic query: card lookup + validation + debit
	result, err := e.store.Cards().AuthorizeAndDebit(ctx, req.PAN, amount)
	if err != nil {
		decision.RespCode = RespSystemMalfunction
		resp.RespCode = decision.RespCode
		reason := "db error"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	// Card not found
	if result == nil {
		decision.RespCode = RespInvalidCardNumber
		resp.RespCode = decision.RespCode
		reason := "card not found"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	// Card found but debit failed — check why
	if !result.Debited {
		// Validate card status/expiry to give specific decline reason
		if rc, reason := ValidateCardBasic(result.Card); rc != "" {
			decision.RespCode = rc
			resp.RespCode = decision.RespCode
			decision.DeclineReason = &reason
			return &resp, decision, nil
		}
		// Card is valid but insufficient funds
		decision.RespCode = RespInsufficientFunds
		resp.RespCode = decision.RespCode
		reason := "insufficient funds"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	decision.RespCode = RespApproved
	resp.RespCode = decision.RespCode
	decision.BalanceBefore = &result.BalanceBefore
	decision.BalanceAfter = &result.BalanceAfter

	authCode := generateAuthCode6()
	decision.AuthCode = &authCode
	resp.AuthRespID = authCode

	return &resp, decision, nil
}

func (e *Engine) handleReversal(ctx context.Context, req *iso.ISOMessage, decision *Decision) (*iso.ISOMessage, *Decision, error) {
	resp := *req
	if len(resp.MTI) == 4 {
		resp.MTI = resp.MTI[:2] + "10"
	}

	amount, err := util.ParseAmount12(req.AmountTrn)
	if err != nil {
		decision.RespCode = RespDoNotHonor
		resp.RespCode = decision.RespCode
		reason := "invalid amount"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	cardRow, err := e.store.Cards().GetCardByPAN(ctx, req.PAN)
	if err != nil || cardRow == nil {
		decision.RespCode = RespSystemMalfunction
		resp.RespCode = decision.RespCode
		reason := "card lookup failed"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	if err := e.store.Cards().CreditBalance(ctx, cardRow.ID, amount); err != nil {
		decision.RespCode = RespSystemMalfunction
		resp.RespCode = decision.RespCode
		reason := "credit failed"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	decision.RespCode = RespApproved
	resp.RespCode = decision.RespCode
	authCode := generateAuthCode6()
	decision.AuthCode = &authCode
	resp.AuthRespID = authCode

	return &resp, decision, nil
}

func generateAuthCode6() string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	for i := 0; i < 6; i++ {
		buf[i] = '0' + (buf[i] % 10)
	}
	return string(buf)
}
