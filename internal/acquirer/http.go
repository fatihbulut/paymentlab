package acquirer

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"iso-parser-service/internal/iso"
)

type HTTPServer struct {
	issuerClient   IssuerClient
	switchInstance *AcquirerSwitch
}

func NewHTTPServer(client IssuerClient, switchInstance *AcquirerSwitch) *HTTPServer {
	return &HTTPServer{
		issuerClient:   client,
		switchInstance: switchInstance,
	}
}

// Router constructs a gin.Engine with all acquirer HTTP routes registered.
func (s *HTTPServer) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Serve UI
	router.GET("/", func(c *gin.Context) {
		c.File("web/index.html")
	})

	// Serve spec.json (Single Source of Truth)
	router.GET("/spec.json", func(c *gin.Context) {
		c.File("web/spec.json")
	})

	// Serve ISO engine
	router.GET("/iso-engine.js", func(c *gin.Context) {
		c.File("web/iso-engine.js")
	})

	// Serve social media image
	router.GET("/og-image.png", func(c *gin.Context) {
		c.Header("Content-Type", "image/png")
		c.File("web/og-image.png")
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/v1/transaction", s.handleTransaction)

	return router
}

func (s *HTTPServer) handleTransaction(c *gin.Context) {
	// Start root trace for HTTP request
	tracer := otel.Tracer("acquirer-tracer")
	ctx, span := tracer.Start(c.Request.Context(), "transaction-processing")
	defer span.End()

	start := time.Now()
	var req iso.ISOMessage

	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "invalid_json"),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ISOMessage JSON"})
		return
	}

	hexReq, err := iso.PackMessageToHex(&req)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "iso_pack_error"),
		)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO pack error: %v", err)})
		return
	}

	// Context propagation: Add trace information to TCP request
	span.SetAttributes(
		attribute.String("transaction.mti", req.MTI),
		attribute.String("transaction.pan_masked", maskPAN(req.PAN)),
	)

	// Convert hex string to raw bytes for Switch
	rawISOBytes, err := hex.DecodeString(hexReq)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "hex_decode_error"),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("hex decode error: %v", err)})
		return
	}

	// Use Switch for TPDU-based communication
	response, err := s.switchInstance.HandleTerminalRequest(ctx, rawISOBytes)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "switch_communication_error"),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("switch communication error: %v", err)})
		return
	}

	// Convert response bytes back to hex string
	hexResp := fmt.Sprintf("%x", response)

	respMsg, err := iso.ParseHexToMessage(hexResp)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "response_parse_error"),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer response parse error: %v", err)})
		return
	}

	// Record metrics
	duration := time.Since(start)
	span.SetAttributes(
		attribute.String("transaction.status", "completed"),
		attribute.String("transaction.response_code", respMsg.RespCode),
		attribute.Float64("transaction.duration_ms", duration.Seconds()*1000),
	)

	// Record critical health metrics
	recordHealthMetrics()

	c.JSON(http.StatusOK, gin.H{
		"request_hex":  hexReq,
		"response_hex": hexResp,
		"response":     respMsg,
	})
}

// maskPAN masks the PAN for logging/tracing
func maskPAN(pan string) string {
	if len(pan) <= 6 {
		return strings.Repeat("*", len(pan))
	}
	if len(pan) <= 10 {
		return pan[:4] + strings.Repeat("*", len(pan)-4)
	}
	return pan[:4] + strings.Repeat("*", len(pan)-8) + pan[len(pan)-4:]
}

// recordHealthMetrics records critical health metrics for monitoring
func recordHealthMetrics() {
	goroutines := runtime.NumGoroutine()
	rss := estimateRSSMemory()
	fmt.Printf("HEALTH_METRIC: service=acquirer goroutines=%d rss_bytes=%d\n", goroutines, rss)
}

// estimateRSSMemory provides a simple RSS memory estimation
func estimateRSSMemory() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys) - int64(m.HeapSys)
}
