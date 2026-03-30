package proccode

import (
	"testing"
)

func TestGetProcessingCode(t *testing.T) {
	tests := []struct {
		name     string
		txType   TransactionType
		fromAcc  AccountType
		toAcc    AccountType
		expected string
	}{
		{
			name:     "Purchase from default account",
			txType:   TxPurchase,
			fromAcc:  AccDefault,
			toAcc:    AccDefault,
			expected: "000000",
		},
		{
			name:     "Purchase from savings",
			txType:   TxPurchase,
			fromAcc:  AccSavings,
			toAcc:    AccDefault,
			expected: "001000",
		},
		{
			name:     "Cash withdrawal from checking",
			txType:   TxCashWithdrawal,
			fromAcc:  AccChecking,
			toAcc:    AccDefault,
			expected: "012000",
		},
		{
			name:     "Balance inquiry on savings",
			txType:   TxBalanceInquiry,
			fromAcc:  AccSavings,
			toAcc:    AccDefault,
			expected: "311000",
		},
		{
			name:     "Purchase with cashback",
			txType:   TxPurchaseCashback,
			fromAcc:  AccDefault,
			toAcc:    AccDefault,
			expected: "090000",
		},
		{
			name:     "Refund to credit card",
			txType:   TxRefund,
			fromAcc:  AccDefault,
			toAcc:    AccCredit,
			expected: "200030",
		},
		{
			name:     "Original credit transfer",
			txType:   TxOriginalCredit,
			fromAcc:  AccDefault,
			toAcc:    AccDefault,
			expected: "260000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetProcessingCode(tt.txType, tt.fromAcc, tt.toAcc)
			if result != tt.expected {
				t.Errorf("GetProcessingCode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateProcessingCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{
			name:    "Valid code",
			code:    "000000",
			wantErr: false,
		},
		{
			name:    "Valid purchase code",
			code:    "001000",
			wantErr: false,
		},
		{
			name:    "Too short",
			code:    "00000",
			wantErr: true,
		},
		{
			name:    "Too long",
			code:    "0000000",
			wantErr: true,
		},
		{
			name:    "Contains letters",
			code:    "00A000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProcessingCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProcessingCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseProcessingCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		want    *ProcessingCode
		wantErr bool
	}{
		{
			name: "Purchase code",
			code: "000000",
			want: &ProcessingCode{
				TransactionType: "00",
				FromAccount:     "00",
				ToAccount:       "00",
			},
			wantErr: false,
		},
		{
			name: "Cash withdrawal from savings",
			code: "011000",
			want: &ProcessingCode{
				TransactionType: "01",
				FromAccount:     "10",
				ToAccount:       "00",
			},
			wantErr: false,
		},
		{
			name:    "Invalid code",
			code:    "00000",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProcessingCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProcessingCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.TransactionType != tt.want.TransactionType ||
					got.FromAccount != tt.want.FromAccount ||
					got.ToAccount != tt.want.ToAccount {
					t.Errorf("ParseProcessingCode() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGetMTIForTransaction(t *testing.T) {
	tests := []struct {
		name       string
		txType     TransactionType
		isResponse bool
		expected   string
	}{
		{
			name:       "Purchase request",
			txType:     TxPurchase,
			isResponse: false,
			expected:   "0100",
		},
		{
			name:       "Purchase response",
			txType:     TxPurchase,
			isResponse: true,
			expected:   "0110",
		},
		{
			name:       "Reversal request",
			txType:     TxReversal,
			isResponse: false,
			expected:   "0400",
		},
		{
			name:       "Reversal response",
			txType:     TxReversal,
			isResponse: true,
			expected:   "0410",
		},
		{
			name:       "Balance inquiry request",
			txType:     TxBalanceInquiry,
			isResponse: false,
			expected:   "0100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMTIForTransaction(tt.txType, tt.isResponse)
			if result != tt.expected {
				t.Errorf("GetMTIForTransaction() = %v, want %v", result, tt.expected)
			}
		})
	}
}
