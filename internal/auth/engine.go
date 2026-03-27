package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/store"
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

	// Route reversals to dedicated handler
	if len(req.MTI) == 4 && req.MTI[:2] == "04" {
		return e.handleReversal(ctx, req, decision)
	}

	// Build base response
	resp := *req
	if len(resp.MTI) == 4 {
		resp.MTI = resp.MTI[:2] + "10"
	}

	amount, err := parseAmount12(req.AmountTrn)
	if err != nil {
		decision.RespCode = RespDoNotHonor
		resp.RespCode = decision.RespCode
		reason := "invalid amount"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	cardRow, err := e.store.Cards().GetCardByPAN(ctx, req.PAN)
	if err != nil {
		decision.RespCode = RespSystemMalfunction
		resp.RespCode = decision.RespCode
		reason := "db error"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	if rc, reason := ValidateCardBasic(cardRow); rc != "" {
		decision.RespCode = rc
		resp.RespCode = decision.RespCode
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	before, after, ok, err := e.store.Cards().DebitIfSufficient(ctx, cardRow.ID, amount)
	if err != nil {
		decision.RespCode = RespSystemMalfunction
		resp.RespCode = decision.RespCode
		reason := "debit failed"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}
	if !ok {
		decision.RespCode = RespInsufficientFunds
		resp.RespCode = decision.RespCode
		reason := "insufficient funds"
		decision.DeclineReason = &reason
		return &resp, decision, nil
	}

	decision.RespCode = RespApproved
	resp.RespCode = decision.RespCode
	decision.BalanceBefore = &before
	decision.BalanceAfter = &after

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

	amount, err := parseAmount12(req.AmountTrn)
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

func parseAmount12(s string) (int64, error) {
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

func generateAuthCode6() string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	for i := 0; i < 6; i++ {
		buf[i] = '0' + (buf[i] % 10)
	}
	return string(buf)
}
