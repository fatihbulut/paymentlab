package proccode

import (
	"fmt"
)

// TransactionType represents the type of financial transaction
type TransactionType string

const (
	// Purchase transactions
	TxPurchase         TransactionType = "PURCHASE"          // Goods/Services Purchase
	TxPurchaseCashback TransactionType = "PURCHASE_CASHBACK" // Purchase with Cashback
	TxPurchaseReturn   TransactionType = "PURCHASE_RETURN"   // Purchase Return/Refund

	// Cash transactions
	TxCashWithdrawal TransactionType = "CASH_WITHDRAWAL" // ATM/POS Cash Withdrawal
	TxCashAdvance    TransactionType = "CASH_ADVANCE"    // Cash Advance
	TxCashDeposit    TransactionType = "CASH_DEPOSIT"    // Cash Deposit

	// Balance inquiries
	TxBalanceInquiry TransactionType = "BALANCE_INQUIRY" // Balance Inquiry
	TxMiniStatement  TransactionType = "MINI_STATEMENT"  // Mini Statement

	// Payment transactions
	TxPayment        TransactionType = "PAYMENT"         // Bill Payment
	TxTransfer       TransactionType = "TRANSFER"        // Fund Transfer
	TxOriginalCredit TransactionType = "ORIGINAL_CREDIT" // Original Credit Transfer (OCT)

	// Reversal and adjustment
	TxReversal   TransactionType = "REVERSAL"   // Transaction Reversal
	TxAdjustment TransactionType = "ADJUSTMENT" // Transaction Adjustment

	// Preauthorization
	TxPreAuth           TransactionType = "PREAUTH"            // Preauthorization
	TxPreAuthCompletion TransactionType = "PREAUTH_COMPLETION" // Preauth Completion

	// Refund
	TxRefund TransactionType = "REFUND" // Merchandise Refund

	// Card verification
	TxCardVerification    TransactionType = "CARD_VERIFICATION"    // Card Verification (CVV check)
	TxAccountVerification TransactionType = "ACCOUNT_VERIFICATION" // Account Verification
)

// AccountType represents the account being accessed
type AccountType string

const (
	AccDefault    AccountType = "00" // Default - Unspecified
	AccSavings    AccountType = "10" // Savings Account
	AccChecking   AccountType = "20" // Checking Account
	AccCredit     AccountType = "30" // Credit Account
	AccUniversal  AccountType = "40" // Universal Account
	AccInvestment AccountType = "50" // Investment Account
)

// ProcessingCode represents the 6-digit ISO 8583 Field 3
// Format: TTFFTT
// - TT (positions 1-2): Transaction Type
// - FF (positions 3-4): From Account Type
// - TT (positions 5-6): To Account Type
type ProcessingCode struct {
	TransactionType string // 2 digits
	FromAccount     string // 2 digits
	ToAccount       string // 2 digits
}

// String returns the 6-digit processing code
func (pc ProcessingCode) String() string {
	return fmt.Sprintf("%s%s%s", pc.TransactionType, pc.FromAccount, pc.ToAccount)
}

// ProcessingCodeSpec defines Mastercard and Visa compliant processing codes
type ProcessingCodeSpec struct {
	Code        string
	Description string
	Mastercard  bool   // Supported by Mastercard
	Visa        bool   // Supported by Visa
	MTI         string // Typical MTI used
}

// GetProcessingCode returns the processing code for a given transaction type
// Mastercard and Visa compliant
func GetProcessingCode(txType TransactionType, fromAcc, toAcc AccountType) string {
	// Default account types if not specified
	if fromAcc == "" {
		fromAcc = AccDefault
	}
	if toAcc == "" {
		toAcc = AccDefault
	}

	var txCode string

	switch txType {
	// Purchase transactions (00xxxx)
	case TxPurchase:
		txCode = "00" // Goods and Services
	case TxPurchaseCashback:
		txCode = "09" // Purchase with Cashback
	case TxPurchaseReturn:
		txCode = "20" // Refund/Return

	// Cash transactions (01xxxx)
	case TxCashWithdrawal:
		txCode = "01" // Cash Withdrawal
	case TxCashAdvance:
		txCode = "01" // Cash Advance (same as withdrawal)
	case TxCashDeposit:
		txCode = "21" // Cash Deposit

	// Balance inquiry (31xxxx)
	case TxBalanceInquiry:
		txCode = "31" // Balance Inquiry
	case TxMiniStatement:
		txCode = "38" // Mini Statement

	// Payment and transfer (40xxxx, 26xxxx)
	case TxPayment:
		txCode = "40" // Payment
	case TxTransfer:
		txCode = "40" // Fund Transfer
	case TxOriginalCredit:
		txCode = "26" // Original Credit Transfer (OCT)

	// Reversal (02xxxx for purchase reversal, 22xxxx for cash reversal)
	case TxReversal:
		txCode = "02" // Reversal (typically used with MTI 0400/0420)

	// Adjustment
	case TxAdjustment:
		txCode = "02" // Adjustment

	// Preauthorization (00xxxx with specific handling)
	case TxPreAuth:
		txCode = "00" // Preauth uses same as purchase
	case TxPreAuthCompletion:
		txCode = "00" // Completion uses same as purchase

	// Refund (20xxxx)
	case TxRefund:
		txCode = "20" // Merchandise Refund

	// Card/Account verification (00xxxx with zero amount)
	case TxCardVerification:
		txCode = "00" // Card Verification
	case TxAccountVerification:
		txCode = "93" // Account Verification

	default:
		txCode = "00" // Default to purchase
	}

	return fmt.Sprintf("%s%s%s", txCode, fromAcc, toAcc)
}

// GetMTIForTransaction returns the appropriate MTI for a transaction type
func GetMTIForTransaction(txType TransactionType, isResponse bool) string {
	if isResponse {
		switch txType {
		case TxReversal:
			return "0410" // Reversal response
		default:
			return "0110" // Authorization response
		}
	}

	switch txType {
	case TxReversal:
		return "0400" // Reversal request
	case TxBalanceInquiry, TxMiniStatement:
		return "0100" // Financial transaction request
	default:
		return "0100" // Authorization request
	}
}

// AllTransactionTypes returns all supported transaction types with descriptions
func AllTransactionTypes() []struct {
	Type        TransactionType
	Code        string
	Description string
	Mastercard  bool
	Visa        bool
} {
	return []struct {
		Type        TransactionType
		Code        string
		Description string
		Mastercard  bool
		Visa        bool
	}{
		{TxPurchase, "00", "Goods/Services Purchase", true, true},
		{TxPurchaseCashback, "09", "Purchase with Cashback", true, true},
		{TxPurchaseReturn, "20", "Purchase Return/Refund", true, true},
		{TxCashWithdrawal, "01", "Cash Withdrawal", true, true},
		{TxCashAdvance, "01", "Cash Advance", true, true},
		{TxCashDeposit, "21", "Cash Deposit", true, true},
		{TxBalanceInquiry, "31", "Balance Inquiry", true, true},
		{TxMiniStatement, "38", "Mini Statement", true, false},
		{TxPayment, "40", "Bill Payment", true, true},
		{TxTransfer, "40", "Fund Transfer", true, true},
		{TxOriginalCredit, "26", "Original Credit Transfer", true, true},
		{TxReversal, "02", "Transaction Reversal", true, true},
		{TxAdjustment, "02", "Transaction Adjustment", true, true},
		{TxPreAuth, "00", "Preauthorization", true, true},
		{TxPreAuthCompletion, "00", "Preauth Completion", true, true},
		{TxRefund, "20", "Merchandise Refund", true, true},
		{TxCardVerification, "00", "Card Verification", true, true},
		{TxAccountVerification, "93", "Account Verification", true, true},
	}
}

// ValidateProcessingCode validates a 6-digit processing code
func ValidateProcessingCode(code string) error {
	if len(code) != 6 {
		return fmt.Errorf("processing code must be 6 digits, got %d", len(code))
	}

	for _, c := range code {
		if c < '0' || c > '9' {
			return fmt.Errorf("processing code must contain only digits")
		}
	}

	return nil
}

// ParseProcessingCode parses a 6-digit processing code into its components
func ParseProcessingCode(code string) (*ProcessingCode, error) {
	if err := ValidateProcessingCode(code); err != nil {
		return nil, err
	}

	return &ProcessingCode{
		TransactionType: code[0:2],
		FromAccount:     code[2:4],
		ToAccount:       code[4:6],
	}, nil
}

// GetTransactionDescription returns a human-readable description of the transaction
func GetTransactionDescription(txType TransactionType) string {
	descriptions := map[TransactionType]string{
		TxPurchase:            "Goods/Services Purchase",
		TxPurchaseCashback:    "Purchase with Cashback",
		TxPurchaseReturn:      "Purchase Return/Refund",
		TxCashWithdrawal:      "Cash Withdrawal",
		TxCashAdvance:         "Cash Advance",
		TxCashDeposit:         "Cash Deposit",
		TxBalanceInquiry:      "Balance Inquiry",
		TxMiniStatement:       "Mini Statement",
		TxPayment:             "Bill Payment",
		TxTransfer:            "Fund Transfer",
		TxOriginalCredit:      "Original Credit Transfer (OCT)",
		TxReversal:            "Transaction Reversal",
		TxAdjustment:          "Transaction Adjustment",
		TxPreAuth:             "Preauthorization",
		TxPreAuthCompletion:   "Preauthorization Completion",
		TxRefund:              "Merchandise Refund",
		TxCardVerification:    "Card Verification",
		TxAccountVerification: "Account Verification",
	}

	if desc, ok := descriptions[txType]; ok {
		return desc
	}
	return "Unknown Transaction Type"
}
