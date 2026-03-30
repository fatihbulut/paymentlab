package iso

import (
	"github.com/moov-io/iso8583"
	"github.com/moov-io/iso8583/encoding"
	"github.com/moov-io/iso8583/field"
	"github.com/moov-io/iso8583/prefix"
)

var Spec = &iso8583.MessageSpec{
	Fields: map[int]field.Field{
		0:  field.NewString(&field.Spec{Length: 4, Description: "MTI", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		1:  field.NewBitmap(&field.Spec{Description: "Bitmap", Enc: encoding.Binary, Pref: prefix.ASCII.Fixed}),
		2:  field.NewString(&field.Spec{Length: 19, Description: "PAN", Enc: encoding.ASCII, Pref: prefix.ASCII.LL}),
		3:  field.NewString(&field.Spec{Length: 6, Description: "Processing Code", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		4:  field.NewString(&field.Spec{Length: 12, Description: "Amount, Transaction", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		5:  field.NewString(&field.Spec{Length: 12, Description: "Amount, Settlement", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		6:  field.NewString(&field.Spec{Length: 12, Description: "Amount, Cardholder Billing", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		7:  field.NewString(&field.Spec{Length: 10, Description: "Transmission Date & Time", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		9:  field.NewString(&field.Spec{Length: 8, Description: "Conversion Rate, Settlement", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		10: field.NewString(&field.Spec{Length: 8, Description: "Conversion Rate, Cardholder Billing", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		11: field.NewString(&field.Spec{Length: 6, Description: "STAN", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		12: field.NewString(&field.Spec{Length: 6, Description: "Local Transaction Time", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		13: field.NewString(&field.Spec{Length: 4, Description: "Local Transaction Date", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		14: field.NewString(&field.Spec{Length: 4, Description: "Expiration Date", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		15: field.NewString(&field.Spec{
			Length:      4,
			Description: "Date, Settlement",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		16: field.NewString(&field.Spec{
			Length:      4,
			Description: "Date, Conversion",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		18: field.NewString(&field.Spec{Length: 4, Description: "Merchant Type", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		22: field.NewString(&field.Spec{Length: 3, Description: "Point of Service Entry Mode", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		25: field.NewString(&field.Spec{Length: 2, Description: "Point of Service Condition Code", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		26: field.NewString(&field.Spec{
			Length:      2,
			Description: "Point of Service PIN Capture Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		32: field.NewString(&field.Spec{Length: 11, Description: "Acquiring Institution ID", Enc: encoding.ASCII, Pref: prefix.ASCII.LL}),
		35: field.NewString(&field.Spec{Length: 37, Description: "Track 2 Data", Enc: encoding.ASCII, Pref: prefix.ASCII.LL}),
		37: field.NewString(&field.Spec{Length: 12, Description: "Retrieval Reference Number (RRN)", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		38: field.NewString(&field.Spec{Length: 6, Description: "Authorization Identification Response", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		39: field.NewString(&field.Spec{Length: 2, Description: "Response Code", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		41: field.NewString(&field.Spec{Length: 8, Description: "Card Acceptor Terminal ID", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		42: field.NewString(&field.Spec{Length: 15, Description: "Card Acceptor ID Code", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		43: field.NewString(&field.Spec{Length: 40, Description: "Card Acceptor Name/Location", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		47: field.NewString(&field.Spec{
			Length:      999,
			Description: "Additional Data - National",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
		48: field.NewString(&field.Spec{Length: 999, Description: "Additional Data - Private", Enc: encoding.ASCII, Pref: prefix.ASCII.LLL}),
		49: field.NewString(&field.Spec{Length: 3, Description: "Currency Code, Transaction", Enc: encoding.ASCII, Pref: prefix.ASCII.Fixed}),
		51: field.NewString(&field.Spec{
			Length:      3,
			Description: "Currency Code, Cardholder Billing",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		52: field.NewString(&field.Spec{
			Length:      16,
			Description: "PIN Data",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		53: field.NewString(&field.Spec{
			Length:      16,
			Description: "Security Related Control Info",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		54: field.NewString(&field.Spec{
			Length:      120,
			Description: "Additional Amounts",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
		55: field.NewString(&field.Spec{
			Length:      999,
			Description: "ICC System Related Data (EMV)",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
		61: field.NewString(&field.Spec{
			Length:      999,
			Description: "Terminal Private Data",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
		63: field.NewString(&field.Spec{
			Length:      999,
			Description: "Reserved (private)",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
		70: field.NewString(&field.Spec{
			Length:      3,
			Description: "MW Management Info Code",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.Fixed,
		}),
		125: field.NewString(&field.Spec{
			Length:      999,
			Description: "Reserved (private)",
			Enc:         encoding.ASCII,
			Pref:        prefix.ASCII.LLL,
		}),
	},
}
