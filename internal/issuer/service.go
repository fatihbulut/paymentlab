package issuer

import (
	"context"
	"fmt"
	"strings"

	"iso-parser-service/internal/auth"
	"iso-parser-service/internal/iso"
	"iso-parser-service/internal/store"
)

type Service struct {
	store store.Store
	auth  *auth.Engine
}

func NewService(appStore store.Store) *Service {
	var engine *auth.Engine
	if appStore != nil {
		engine = auth.NewEngine(appStore)
	}
	return &Service{store: appStore, auth: engine}
}

// HandleHex processes a single ISO8583 hex request and returns the hex response
// along with the parsed response message.
func (s *Service) HandleHex(hexReq string) (string, *iso.ISOMessage, error) {
	reqMsg, err := iso.ParseHexToMessage(hexReq)
	if err != nil {
		return "", nil, fmt.Errorf("parse request: %w", err)
	}

	respMsg := decideResponse(reqMsg)

	var issuerTxID *string
	if s.store != nil {
		amount, amountErr := parseAmount12(reqMsg.AmountTrn)
		if amountErr != nil {
			amount = 0
		}
		tx := &store.IssuerTransaction{
			STAN:         strings.TrimSpace(reqMsg.STAN),
			RRN:          nil,
			MTI:          strings.TrimSpace(reqMsg.MTI),
			PANMasked:    maskPAN(reqMsg.PAN),
			Amount:       amount,
			CurrencyCode: strings.TrimSpace(reqMsg.CurCodeTrn),
			Status:       store.TransactionStatus("RECEIVED"),
		}

		created, createErr := s.store.IssuerTransactions().CreateIssuerTransaction(context.Background(), tx)
		if createErr == nil && created != nil {
			id := created.ID
			issuerTxID = &id
		}
	}

	if s.auth != nil {
		authResp, decision, authErr := s.auth.Authorize(context.Background(), reqMsg)
		if authErr == nil && authResp != nil {
			respMsg = authResp
		}

		if s.store != nil && issuerTxID != nil {
			status := store.TransactionStatus("DECLINED")
			if respMsg != nil && respMsg.RespCode == auth.RespApproved {
				status = store.TransactionStatus("APPROVED")
			}
			var rc *string
			if respMsg != nil && strings.TrimSpace(respMsg.RespCode) != "" {
				v := strings.TrimSpace(respMsg.RespCode)
				rc = &v
			}
			var authCode *string
			var declineReason *string
			var balanceBefore *int64
			var balanceAfter *int64
			var durationMs *float64
			if decision != nil {
				authCode = decision.AuthCode
				declineReason = decision.DeclineReason
				balanceBefore = decision.BalanceBefore
				balanceAfter = decision.BalanceAfter
				d := decision.DurationMs
				durationMs = &d
			}
			_ = s.store.IssuerTransactions().UpdateIssuerTransaction(
				context.Background(),
				*issuerTxID,
				status,
				rc,
				authCode,
				declineReason,
				balanceBefore,
				balanceAfter,
				durationMs,
			)
		}
	}

	hexResp, err := iso.PackMessageToHex(respMsg)
	if err != nil {
		return "", nil, fmt.Errorf("pack response: %w", err)
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
