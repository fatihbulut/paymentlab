package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	transactionCounter metric.Int64Counter
	processingDuration metric.Float64Histogram
)

// initBusinessMetrics initializes custom business metrics for payment processing
func initBusinessMetrics() error {
	meter := otel.Meter("payment-lab")

	var err error

	// Counter for total transactions
	// Low cardinality: only mti (limited set) and response_code (limited set)
	transactionCounter, err = meter.Int64Counter(
		"payment_transactions_total",
		metric.WithDescription("Total number of payment transactions processed"),
		metric.WithUnit("{transaction}"),
	)
	if err != nil {
		return err
	}

	// Histogram for processing duration
	// Buckets optimized for payment processing (typically 1-100ms)
	processingDuration, err = meter.Float64Histogram(
		"payment_processing_duration_ms",
		metric.WithDescription("Payment transaction processing duration in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 2, 5, 10, 20, 50, 100, 200, 500, 1000),
	)
	if err != nil {
		return err
	}

	return nil
}

// RecordTransaction records a payment transaction with its metrics
// mti: Message Type Indicator (e.g., "0100", "0110", "0200", "0210")
// responseCode: Response code (e.g., "00", "05", "51", etc.)
// duration: Processing duration
//
// Note: Keep attribute cardinality low for 100K RPS:
// - MTI: ~10 unique values
// - Response codes: ~20 unique values
// Total cardinality: ~200 time series (very manageable)
func RecordTransaction(ctx context.Context, mti string, responseCode string, duration time.Duration) {
	if transactionCounter == nil || processingDuration == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("mti", mti),
		attribute.String("response_code", responseCode),
	}

	// Increment transaction counter
	transactionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record processing duration in milliseconds
	durationMs := float64(duration.Milliseconds())
	processingDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}

// RecordTransactionWithService records a transaction with service name attribute
// Useful for distinguishing between acquirer and issuer metrics
func RecordTransactionWithService(ctx context.Context, service string, mti string, responseCode string, duration time.Duration) {
	if transactionCounter == nil || processingDuration == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("service", service),
		attribute.String("mti", mti),
		attribute.String("response_code", responseCode),
	}

	transactionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	durationMs := float64(duration.Milliseconds())
	processingDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}
