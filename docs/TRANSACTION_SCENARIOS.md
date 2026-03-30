# Transaction Scenarios Guide

## Overview

This guide provides comprehensive transaction scenarios for testing your ISO 8583 payment simulator. Each scenario includes the complete message structure with appropriate MTI, processing codes, and field values.

## Scenario Categories

### 1. Purchase Transactions

#### 1.1 Standard POS Purchase
```
Transaction Type: Purchase
MTI: 0100 (Request) / 0110 (Response)
Processing Code: 000000
Channel: POS
Entry Mode: 051 (Chip)

Key Fields:
- Field 3: 000000 (Purchase from default account)
- Field 4: 000000015000 (150.00 TRY)
- Field 22: 051 (ICC/Chip read)
- Field 25: 00 (Normal presentment)
- Field 49: 949 (TRY)
- Field 55: ICC Data (EMV chip data)

Expected Response Code: 00 (Approved)
```

#### 1.2 Purchase with Cashback
```
Transaction Type: Purchase with Cashback
MTI: 0100 / 0110
Processing Code: 090000
Channel: POS

Key Fields:
- Field 3: 090000 (Purchase with cashback)
- Field 4: 000000020000 (200.00 TRY - total amount)
- Field 54: Additional amounts (cashback amount)
- Field 22: 051
- Field 25: 00

Expected Response Code: 00 (Approved)
```

#### 1.3 E-commerce Purchase
```
Transaction Type: Purchase
MTI: 0100 / 0110
Processing Code: 000000
Channel: Web
Entry Mode: 811 (E-commerce)

Key Fields:
- Field 3: 000000
- Field 4: 000000050000 (500.00 TRY)
- Field 22: 811 (E-commerce transaction)
- Field 25: 59 (E-commerce)
- Field 49: 949

Expected Response Code: 00 (Approved)
```

### 2. Cash Transactions

#### 2.1 ATM Cash Withdrawal from Savings
```
Transaction Type: Cash Withdrawal
MTI: 0100 / 0110
Processing Code: 011000
Channel: ATM
Entry Mode: 021 (Magnetic stripe)

Key Fields:
- Field 3: 011000 (Cash withdrawal from savings)
- Field 4: 000000050000 (500.00 TRY)
- Field 22: 021 (Magnetic stripe)
- Field 25: 00
- Field 49: 949

Expected Response Code: 00 (Approved) or 51 (Insufficient funds)
```

#### 2.2 Cash Withdrawal from Checking
```
Transaction Type: Cash Withdrawal
MTI: 0100 / 0110
Processing Code: 012000
Channel: ATM

Key Fields:
- Field 3: 012000 (Cash withdrawal from checking)
- Field 4: 000000020000 (200.00 TRY)
- Field 22: 021
- Field 25: 00

Expected Response Code: 00 (Approved)
```

#### 2.3 Cash Deposit
```
Transaction Type: Cash Deposit
MTI: 0100 / 0110
Processing Code: 210000
Channel: ATM

Key Fields:
- Field 3: 210000 (Cash deposit to default account)
- Field 4: 000000100000 (1000.00 TRY)
- Field 22: 021
- Field 25: 00

Expected Response Code: 00 (Approved)
```

### 3. Balance Inquiry

#### 3.1 Balance Inquiry on Savings
```
Transaction Type: Balance Inquiry
MTI: 0100 / 0110
Processing Code: 311000
Channel: ATM
Amount: 0 (Zero amount transaction)

Key Fields:
- Field 3: 311000 (Balance inquiry on savings)
- Field 4: 000000000000 (Zero amount)
- Field 22: 021
- Field 25: 00
- Field 54: Balance amounts in response

Expected Response Code: 00 (Approved)
```

#### 3.2 Mini Statement
```
Transaction Type: Mini Statement
MTI: 0100 / 0110
Processing Code: 380000
Channel: ATM

Key Fields:
- Field 3: 380000 (Mini statement)
- Field 4: 000000000000 (Zero amount)
- Field 22: 021
- Field 25: 00

Expected Response Code: 00 (Approved)
Note: Mastercard only, limited Visa support
```

### 4. Refund & Return Transactions

#### 4.1 Merchandise Refund
```
Transaction Type: Refund
MTI: 0100 / 0110
Processing Code: 200000
Channel: POS

Key Fields:
- Field 3: 200000 (Refund to default account)
- Field 4: 000000015000 (150.00 TRY)
- Field 22: 051
- Field 25: 00
- Field 37: Original RRN (if available)

Expected Response Code: 00 (Approved)
```

#### 4.2 Refund to Credit Card
```
Transaction Type: Refund
MTI: 0100 / 0110
Processing Code: 200030
Channel: POS

Key Fields:
- Field 3: 200030 (Refund to credit account)
- Field 4: 000000025000 (250.00 TRY)
- Field 22: 051
- Field 25: 00

Expected Response Code: 00 (Approved)
```

### 5. Reversal Transactions

#### 5.1 Purchase Reversal
```
Transaction Type: Reversal
MTI: 0400 (Request) / 0410 (Response)
Processing Code: 020000
Channel: POS

Key Fields:
- Field 3: 020000 (Reversal)
- Field 4: 000000015000 (Original amount)
- Field 11: Original STAN
- Field 37: Original RRN
- Field 90: Original data elements

Expected Response Code: 00 (Approved)
```

#### 5.2 Cash Withdrawal Reversal
```
Transaction Type: Reversal
MTI: 0400 / 0410
Processing Code: 020000
Channel: ATM

Key Fields:
- Field 3: 020000
- Field 4: 000000050000 (Original amount)
- Field 11: Original STAN
- Field 37: Original RRN

Expected Response Code: 00 (Approved)
```

### 6. Payment & Transfer

#### 6.1 Bill Payment
```
Transaction Type: Payment
MTI: 0100 / 0110
Processing Code: 400000
Channel: Web

Key Fields:
- Field 3: 400000 (Payment from default account)
- Field 4: 000000035000 (350.00 TRY)
- Field 22: 811
- Field 25: 00
- Field 48: Biller information

Expected Response Code: 00 (Approved)
```

#### 6.2 Fund Transfer (Savings to Checking)
```
Transaction Type: Transfer
MTI: 0100 / 0110
Processing Code: 401020
Channel: ATM

Key Fields:
- Field 3: 401020 (Transfer from savings to checking)
- Field 4: 000000100000 (1000.00 TRY)
- Field 22: 021
- Field 25: 00

Expected Response Code: 00 (Approved)
```

#### 6.3 Original Credit Transfer (P2P)
```
Transaction Type: Original Credit Transfer
MTI: 0100 / 0110
Processing Code: 260000
Channel: Web

Key Fields:
- Field 3: 260000 (OCT - money send)
- Field 4: 000000050000 (500.00 TRY)
- Field 22: 811
- Field 25: 00
- Field 102: Recipient account ID

Expected Response Code: 00 (Approved)
```

### 7. Preauthorization

#### 7.1 Hotel Preauthorization
```
Transaction Type: Preauthorization
MTI: 0100 / 0110
Processing Code: 000000
Channel: POS

Key Fields:
- Field 3: 000000 (Preauth uses purchase code)
- Field 4: 000000100000 (1000.00 TRY - estimated)
- Field 18: 7011 (Hotel merchant category)
- Field 22: 051
- Field 25: 00

Expected Response Code: 00 (Approved)
Note: Funds are held but not captured
```

#### 7.2 Preauthorization Completion
```
Transaction Type: Preauth Completion
MTI: 0220 (Completion advice)
Processing Code: 000000
Channel: POS

Key Fields:
- Field 3: 000000
- Field 4: 000000085000 (850.00 TRY - actual amount)
- Field 38: Original auth code
- Field 22: 051

Expected Response Code: 00 (Approved)
```

### 8. Verification Transactions

#### 8.1 Card Verification (Zero Amount)
```
Transaction Type: Card Verification
MTI: 0100 / 0110
Processing Code: 000000
Channel: Web
Amount: 0

Key Fields:
- Field 3: 000000
- Field 4: 000000000000 (Zero amount)
- Field 22: 811
- Field 25: 59

Expected Response Code: 00 (Approved) or 14 (Invalid card)
```

#### 8.2 Account Verification
```
Transaction Type: Account Verification
MTI: 0100 / 0110
Processing Code: 930000
Channel: ATM

Key Fields:
- Field 3: 930000 (Account verification)
- Field 4: 000000000000 (Zero amount)
- Field 22: 021
- Field 25: 00

Expected Response Code: 00 (Approved)
```

## Decline Scenarios

### Insufficient Funds
```
Processing Code: 000000 (Purchase)
Amount: Very high (exceeds balance)
Expected Response Code: 51 (Insufficient funds)
```

### Expired Card
```
Processing Code: 000000
Card with past expiry date
Expected Response Code: 54 (Expired card)
```

### Invalid Card Number
```
Processing Code: 000000
Invalid PAN (fails Luhn check)
Expected Response Code: 14 (Invalid card number)
```

### Exceeds Withdrawal Limit
```
Processing Code: 011000 (Cash withdrawal)
Amount: Exceeds daily limit
Expected Response Code: 61 (Exceeds withdrawal limit)
```

## Testing Workflow

1. **Create Test Cards**: Use the Cards page to create test cards with different schemes (Visa, Mastercard, Troy)
2. **Set Balances**: Top up cards with appropriate balances
3. **Select Transaction Type**: Choose from the transaction type dropdown
4. **Configure Accounts**: Select from/to account types as needed
5. **Set Amount**: Enter transaction amount
6. **Process**: Click Process to send the transaction
7. **Verify Response**: Check response code and message details

## Response Code Reference

| Code | Description | Scenario |
|------|-------------|----------|
| 00 | Approved | Successful transaction |
| 05 | Do not honor | Generic decline |
| 14 | Invalid card number | Card validation failed |
| 51 | Insufficient funds | Balance too low |
| 54 | Expired card | Card past expiry date |
| 55 | Incorrect PIN | PIN verification failed |
| 57 | Transaction not permitted | Card restrictions |
| 61 | Exceeds withdrawal limit | Over daily limit |
| 91 | Issuer unavailable | System error |
| 96 | System malfunction | Processing error |

## Advanced Scenarios

### Multi-Currency Transaction
```
Processing Code: 000000
Field 49: 949 (TRY - Transaction currency)
Field 51: 840 (USD - Billing currency)
Field 6: Billing amount in USD
Field 10: Conversion rate
```

### EMV Chip Transaction
```
Processing Code: 000000
Field 22: 051 (ICC/Chip)
Field 55: Full EMV data (9F26, 9F27, 9F10, etc.)
```

### Contactless Transaction
```
Processing Code: 000000
Field 22: 071 (Contactless chip)
Field 55: Contactless EMV data
```

## Notes

- All amounts are in minor units (kuruş for TRY)
- STAN (Field 11) is auto-generated for each transaction
- RRN (Field 37) is generated by the issuer in response
- Terminal ID (Field 41) changes based on channel selection
- Timestamps (Fields 7, 12, 13) are auto-generated

## Simulator Features

The ISO 8583 Payment Simulator supports:
- ✓ All transaction types listed above
- ✓ Dynamic processing code generation
- ✓ Mastercard and Visa compliance indicators
- ✓ Real-time message encoding/decoding
- ✓ Hex, ASCII, and JSON views
- ✓ Transaction history tracking
- ✓ Card management
- ✓ Balance tracking
- ✓ EMV/ICC data support
