package main

import "github.com/moov-io/iso8583/field"

type MyMessage struct {
	MTI           *field.String `iso8583:"0"`
	Bitmap        *field.Bitmap `iso8583:"1"`
	PAN           *field.String `iso8583:"2"`
	ProcCode      *field.String `iso8583:"3"`
	AmountTrn     *field.String `iso8583:"4"`
	AmountSet     *field.String `iso8583:"5"`
	AmountBil     *field.String `iso8583:"6"`
	TransDateTime *field.String `iso8583:"7"`
	ConvRateSet   *field.String `iso8583:"9"`
	ConvRateBil   *field.String `iso8583:"10"`
	STAN          *field.String `iso8583:"11"`
	LocalTime     *field.String `iso8583:"12"`
	LocalDate     *field.String `iso8583:"13"`
	ExpDate       *field.String `iso8583:"14"`
	DateSet       *field.String `iso8583:"15"`
	DateConv      *field.String `iso8583:"16"`
	MerchantType  *field.String `iso8583:"18"`
	POSEntryMode  *field.String `iso8583:"22"`
	POSCondCode   *field.String `iso8583:"25"`
	POSPinCode    *field.String `iso8583:"26"`
	AcqInstID     *field.String `iso8583:"32"`
	Track2Data    *field.String `iso8583:"35"`
	RRN           *field.String `iso8583:"37"`
	AuthRespID    *field.String `iso8583:"38"`
	RespCode      *field.String `iso8583:"39"`
	TerminalID    *field.String `iso8583:"41"`
	MerchantID    *field.String `iso8583:"42"`
	MerchantLoc   *field.String `iso8583:"43"`
	AddDataNat    *field.String `iso8583:"47"`
	AddDataPriv   *field.String `iso8583:"48"`
	CurCodeTrn    *field.String `iso8583:"49"`
	CurCodeBil    *field.String `iso8583:"51"`
	PinData       *field.String `iso8583:"52"`
	AddAmounts    *field.String `iso8583:"54"`
	Priv2         *field.String `iso8583:"61"`
}
