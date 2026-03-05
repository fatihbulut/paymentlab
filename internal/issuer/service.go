package issuer

import (
	"fmt"

	"iso-parser-service/internal/iso"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

// HandleHex processes a single ISO8583 hex request and returns the hex response
// along with the parsed response message.
func (s *Service) HandleHex(hexReq string) (string, *iso.ISOMessage, error) {
	reqMsg, err := iso.ParseHexToMessage(hexReq)
	if err != nil {
		return "", nil, fmt.Errorf("parse request: %w", err)
	}

	respMsg := decideResponse(reqMsg)

	hexResp, err := iso.PackMessageToHex(respMsg)
	if err != nil {
		return "", nil, fmt.Errorf("pack response: %w", err)
	}

	return hexResp, respMsg, nil
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
