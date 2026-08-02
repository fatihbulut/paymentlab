#!/bin/bash
# C10K Test Script - Vegeta ile gradual load test
# Mevcut: 2GB RAM (Acquirer 1GB + Issuer 1GB)

# Step 1: C100 Baseline
echo "=== C100 Test (100 req/s, 30s) ==="
echo "POST http://localhost:8081/v1/transaction" | vegeta attack \
  -rate=100 \
  -duration=30s \
  -body=request.json \
  -connections=100 \
  | vegeta report

# Step 2: C500
echo "=== C500 Test ==="
echo "POST http://localhost:8081/v1/transaction" | vegeta attack \
  -rate=500 \
  -duration=30s \
  -body=request.json \
  -connections=500 \
  | vegeta report

# Step 3: C1K (Expected limit with 2GB)
echo "=== C1K Test ==="
echo "POST http://localhost:8081/v1/transaction" | vegeta attack \
  -rate=1000 \
  -duration=30s \
  -body=request.json \
  -connections=1000 \
  | vegeta report

# Step 4: C2K (Will likely fail with OOM)
echo "=== C2K Test (Stress Test) ==="
echo "POST http://localhost:8081/v1/transaction" | vegeta attack \
  -rate=2000 \
  -duration=10s \
  -body=request.json \
  -connections=2000 \
  | vegeta report
