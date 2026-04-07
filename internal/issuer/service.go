package issuer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"iso-parser-service/internal/auth"
	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/store"
)

type Service struct {
	store   store.Store
	auth    *auth.Engine
	auditCh chan *store.IssuerTransaction
}

func NewService(appStore store.Store) *Service {
	var engine *auth.Engine
	if appStore != nil {
		engine = auth.NewEngine(appStore)
	}
	svc := &Service{
		store:   appStore,
		auth:    engine,
		auditCh: make(chan *store.IssuerTransaction, 1000),
	}
	for i := 0; i < 3; i++ {
		go svc.auditWorker()
	}
	return svc
}

func (s *Service) auditWorker() {
	if s.store == nil {
		return
	}
	for tx := range s.auditCh {
		_, _ = s.store.IssuerTransactions().CreateIssuerTransaction(context.Background(), tx)
	}
}

// HandleHex processes a single ISO8583 hex request and returns the hex response
// along with the parsed response message.
func (s *Service) HandleHex(ctx context.Context, hexReq string) (string, *iso.ISOMessage, error) {
	handleStart := time.Now()

	reqMsg, err := iso.ParseHexToMessage(hexReq)
	parseDone := time.Now()
	if err != nil {
		return "", nil, fmt.Errorf("parse request: %w", err)
	}

	respMsg := decideResponse(reqMsg)

	// Auth engine (contains the only sync DB call: AuthorizeAndDebit)
	var decision *auth.Decision
	authStart := time.Now()
	if s.auth != nil {
		authResp, dec, authErr := s.auth.Authorize(ctx, reqMsg)
		decision = dec
		if authErr == nil && authResp != nil {
			respMsg = authResp
		}
	}
	authDone := time.Now()

	hexResp, err := iso.PackMessageToHex(respMsg)
	packDone := time.Now()
	if err != nil {
		return "", nil, fmt.Errorf("pack response: %w", err)
	}

	log.Printf("issuer: handle parse=%dms auth=%dms pack=%dms total=%dms",
		parseDone.Sub(handleStart).Milliseconds(),
		authDone.Sub(authStart).Milliseconds(),
		packDone.Sub(authDone).Milliseconds(),
		time.Since(handleStart).Milliseconds(),
	)

	// Async audit log: single INSERT with final status (non-blocking)
	if s.store != nil {
		amount, amountErr := parseAmount12(reqMsg.AmountTrn)
		if amountErr != nil {
			amount = 0
		}

		status := store.TransactionStatus("DECLINED")
		if respMsg != nil && respMsg.RespCode == auth.RespApproved {
			status = store.TransactionStatus("APPROVED")
		}
		var rc *string
		if respMsg != nil && strings.TrimSpace(respMsg.RespCode) != "" {
			v := strings.TrimSpace(respMsg.RespCode)
			rc = &v
		}
		var authCodePtr *string
		var declineReason *string
		var balanceBefore *int64
		var balanceAfter *int64
		var durationMs *float64
		if decision != nil {
			authCodePtr = decision.AuthCode
			declineReason = decision.DeclineReason
			balanceBefore = decision.BalanceBefore
			balanceAfter = decision.BalanceAfter
			d := decision.DurationMs
			durationMs = &d
		}

		tx := &store.IssuerTransaction{
			STAN:             strings.TrimSpace(reqMsg.STAN),
			MTI:              strings.TrimSpace(reqMsg.MTI),
			PANMasked:        maskPAN(reqMsg.PAN),
			Amount:           amount,
			CurrencyCode:     strings.TrimSpace(reqMsg.CurCodeTrn),
			Status:           status,
			ResponseCode:     rc,
			AuthCode:         authCodePtr,
			DeclineReason:    declineReason,
			BalanceBefore:    balanceBefore,
			BalanceAfter:     balanceAfter,
			ProcessingTimeMs: durationMs,
		}

		select {
		case s.auditCh <- tx:
		default:
		}
	}

	return hexResp, respMsg, nil
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

func maskPAN(pan string) string {
	pan = strings.ReplaceAll(strings.TrimSpace(pan), " ", "")
	if len(pan) <= 6 {
		return strings.Repeat("*", len(pan))
	}
	if len(pan) <= 10 {
		return pan[:4] + strings.Repeat("*", len(pan)-4)
	}
	return pan[:4] + strings.Repeat("*", len(pan)-8) + pan[len(pan)-4:]
}

// decideResponse contains inline, deterministic rules for MVP.
// Later this can delegate to a separate decision engine.
func decideResponse(req *iso.ISOMessage) *iso.ISOMessage {
	resp := *req

	if len(resp.MTI) == 4 {
		resp.MTI = resp.MTI[:2] + "10"
	}

	if resp.AmountTrn == "" {
		resp.RespCode = "05"
	} else {
		resp.RespCode = "00"
	}

	return &resp
}
