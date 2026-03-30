# ISO 8583 Processing Code Guide (Field 3)

## Overview

Processing Code (Field 3) is a 6-digit field that identifies the type of transaction and the accounts involved. This guide provides Mastercard and Visa compliant processing codes for common transaction scenarios.

## Format

```
TTFFTT
```

- **TT** (positions 1-2): Transaction Type Code
- **FF** (positions 3-4): From Account Type
- **TT** (positions 5-6): To Account Type

## Account Type Codes

| Code | Description |
|------|-------------|
| 00   | Default - Unspecified |
| 10   | Savings Account |
| 20   | Checking Account |
| 30   | Credit Account |
| 40   | Universal Account |
| 50   | Investment Account |

## Transaction Type Codes

### Purchase Transactions (00xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 00   | Purchase | Goods and Services Purchase | ✓ | ✓ |
| 09   | Purchase with Cashback | Purchase with Cash Withdrawal | ✓ | ✓ |

**Examples:**
- `000000` - Purchase from default account
- `001000` - Purchase from savings account
- `090000` - Purchase with cashback from default account

### Refund Transactions (20xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 20   | Refund | Merchandise Return/Refund | ✓ | ✓ |

**Examples:**
- `200000` - Refund to default account
- `200030` - Refund to credit card account

### Cash Transactions (01xxxx, 21xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 01   | Cash Withdrawal | ATM/POS Cash Withdrawal | ✓ | ✓ |
| 01   | Cash Advance | Cash Advance on Credit Card | ✓ | ✓ |
| 21   | Cash Deposit | Cash Deposit to Account | ✓ | ✓ |

**Examples:**
- `010000` - Cash withdrawal from default account
- `011000` - Cash withdrawal from savings account
- `012000` - Cash withdrawal from checking account
- `210000` - Cash deposit to default account

### Balance Inquiry (31xxxx, 38xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 31   | Balance Inquiry | Account Balance Inquiry | ✓ | ✓ |
| 38   | Mini Statement | Mini Statement Request | ✓ | ✗ |

**Examples:**
- `310000` - Balance inquiry on default account
- `311000` - Balance inquiry on savings account
- `380000` - Mini statement request

### Payment & Transfer (26xxxx, 40xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 26   | Original Credit Transfer (OCT) | Money Send/P2P Transfer | ✓ | ✓ |
| 40   | Payment/Transfer | Bill Payment or Fund Transfer | ✓ | ✓ |

**Examples:**
- `260000` - Original credit transfer
- `400000` - Bill payment from default account
- `401020` - Transfer from savings to checking

### Reversal & Adjustment (02xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 02   | Reversal | Transaction Reversal | ✓ | ✓ |
| 02   | Adjustment | Transaction Adjustment | ✓ | ✓ |

**Examples:**
- `020000` - Purchase reversal
- `020000` - Transaction adjustment

**Note:** Reversals typically use MTI 0400/0410 instead of 0100/0110

### Preauthorization (00xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 00   | Preauthorization | Hold funds for future completion | ✓ | ✓ |
| 00   | Preauth Completion | Complete a preauthorization | ✓ | ✓ |

**Examples:**
- `000000` - Preauthorization (hotel, car rental)
- `000000` - Preauth completion

**Note:** Preauth uses same processing code as purchase but different MTI handling

### Verification (00xxxx, 93xxxx)

| Code | Transaction Type | Description | Mastercard | Visa |
|------|-----------------|-------------|------------|------|
| 00   | Card Verification | Verify card validity (zero amount) | ✓ | ✓ |
| 93   | Account Verification | Verify account status | ✓ | ✓ |

**Examples:**
- `000000` - Card verification with zero amount
- `930000` - Account verification

## Common Scenarios

### POS Purchase
```
Processing Code: 000000
MTI: 0100 (request) / 0110 (response)
Description: Standard purchase from default account
```

### ATM Withdrawal from Savings
```
Processing Code: 011000
MTI: 0100 (request) / 0110 (response)
Description: Cash withdrawal from savings account
```

### Balance Inquiry at ATM
```
Processing Code: 311000
MTI: 0100 (request) / 0110 (response)
Description: Check savings account balance
```

### Refund to Credit Card
```
Processing Code: 200030
MTI: 0100 (request) / 0110 (response)
Description: Refund merchandise to credit card
```

### Purchase with Cashback
```
Processing Code: 090000
MTI: 0100 (request) / 0110 (response)
Description: Purchase with cash withdrawal
```

### Original Credit Transfer (Money Send)
```
Processing Code: 260000
MTI: 0100 (request) / 0110 (response)
Description: Send money to another card (P2P)
```

## Implementation Notes

### Mastercard Specific
- Supports all transaction types listed above
- OCT (Original Credit Transfer) widely used for disbursements
- Mini statement support varies by market

### Visa Specific
- Does not support mini statement (38xxxx) in all markets
- Strong support for OCT transactions
- Card verification commonly used for e-commerce

### Best Practices

1. **Always validate processing code format**: Exactly 6 digits
2. **Match with MTI**: Ensure processing code aligns with MTI
3. **Account type consistency**: Use appropriate account types for the transaction
4. **Zero amount transactions**: Use for verification only (00xxxx or 93xxxx)
5. **Reversal handling**: Use MTI 0400/0410 with appropriate processing code

## Testing Scenarios

The simulator supports all processing codes listed above. Use the transaction type selector in the UI to automatically generate the correct processing code based on:
- Transaction type (purchase, withdrawal, refund, etc.)
- From account type (savings, checking, credit, etc.)
- To account type (for transfers)

## References

- ISO 8583:1987 Specification
- Mastercard Authorization Manual
- Visa Core Rules and Visa Product and Service Rules
- EMV Specifications

## Simulator Usage

In the ISO 8583 Payment Simulator:

1. Select **Transaction Type** from the dropdown (e.g., "Purchase", "Cash Withdrawal")
2. Select **From Account** type (default, savings, checking, credit)
3. Select **To Account** type (for transfers)
4. The **Processing Code** is automatically generated and displayed
5. Both Mastercard and Visa compatibility indicators are shown

The processing code is dynamically included in Field 3 of the ISO 8583 message.
