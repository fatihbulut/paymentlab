package acquirer

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"iso-parser-service/internal/card"
	"iso-parser-service/internal/iso"
	otellib "iso-parser-service/internal/otel"
	"iso-parser-service/internal/store"
)

type HTTPServer struct {
	switchInstance *AcquirerSwitch
	appStore       store.Store
	limiter        *ConcurrencyLimiter
	auditCh        chan *store.AcquirerTransaction
}

func NewHTTPServer(switchInstance *AcquirerSwitch, appStore store.Store) *HTTPServer {
	srv := &HTTPServer{
		switchInstance: switchInstance,
		appStore:       appStore,
		limiter:        NewConcurrencyLimiter(),
		auditCh:        make(chan *store.AcquirerTransaction, 1000),
	}
	for i := 0; i < 3; i++ {
		go srv.auditWorker()
	}
	return srv
}

func (s *HTTPServer) auditWorker() {
	if s.appStore == nil {
		return
	}
	for tx := range s.auditCh {
		_, _ = s.appStore.AcquirerTransactions().CreateAcquirerTransaction(context.Background(), tx)
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
		c.JSON(http.StatusOK, gin.H{
			"status":         "ok",
			"active":         s.limiter.Active(),
			"queued":         s.limiter.Queued(),
			"max_concurrent": s.limiter.MaxConcurrent(),
			"max_queue":      s.limiter.MaxQueue(),
		})
	})

	router.POST("/v1/cards", s.handleCreateCard)
	router.GET("/v1/cards", s.handleListCards)
	router.PUT("/v1/cards/:id", s.handleUpdateCard)
	router.DELETE("/v1/cards/:id", s.handleDeleteCard)
	router.POST("/v1/cards/:id/topup", s.handleTopUpCard)
	router.POST("/v1/transaction", s.limiter.Middleware(), s.handleTransaction)
	router.GET("/v1/transactions", s.handleListTransactions)
	router.GET("/v1/issuer-transactions", s.handleListIssuerTransactions)

	return router
}

func (s *HTTPServer) handleCreateCard(c *gin.Context) {
	if s.appStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
		return
	}

	var req card.CreateCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request JSON"})
		return
	}

	svc := card.NewService(s.appStore)
	resp, err := svc.CreateCard(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (s *HTTPServer) handleListCards(c *gin.Context) {
	if s.appStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
		return
	}

	limit := 50
	offset := 0
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	svc := card.NewService(s.appStore)
	resp, err := svc.ListCards(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cards": resp})
}

func (s *HTTPServer) handleUpdateCard(c *gin.Context) {
	if s.appStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
		return
	}
	id := c.Param("id")
	var req card.UpdateCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request JSON"})
		return
	}
	svc := card.NewService(s.appStore)
	resp, err := svc.UpdateCard(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *HTTPServer) handleDeleteCard(c *gin.Context) {
	if s.appStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
		return
	}
	id := c.Param("id")
	svc := card.NewService(s.appStore)
	if err := svc.DeleteCard(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
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
	packDone := time.Now()
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
	switchStart := time.Now()
	response, err := s.switchInstance.HandleTerminalRequest(ctx, rawISOBytes)
	switchDone := time.Now()
	if err != nil {
		// Async audit log for error case
		s.asyncLogAcquirerTx(&req, &hexReq, nil, store.TransactionStatus("ERROR"))
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
		// Async audit log for parse error
		s.asyncLogAcquirerTx(&req, &hexReq, &hexResp, store.TransactionStatus("ERROR"))
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "response_parse_error"),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer response parse error: %v", err)})
		return
	}

	// Async audit log: single INSERT with final status (non-blocking)
	{
		status := store.TransactionStatus("DECLINED")
		if respMsg.RespCode == "00" {
			status = store.TransactionStatus("APPROVED")
		}
		rc := respMsg.RespCode
		s.asyncLogAcquirerTx(&req, &hexReq, &hexResp, status, &rc)
	}

	parseDone := time.Now()

	// Timing log
	queueWait := float64(0)
	if v, exists := c.Get("queue_wait_ms"); exists {
		queueWait = v.(float64)
	}
	log.Printf("acquirer: txn queue=%.0fms pack=%dms switch_rtt=%dms parse=%dms total=%dms",
		queueWait,
		packDone.Sub(start).Milliseconds(),
		switchDone.Sub(switchStart).Milliseconds(),
		parseDone.Sub(switchDone).Milliseconds(),
		time.Since(start).Milliseconds(),
	)

	// Record metrics
	duration := time.Since(start)
	span.SetAttributes(
		attribute.String("transaction.status", "completed"),
		attribute.String("transaction.response_code", respMsg.RespCode),
		attribute.Float64("transaction.duration_ms", duration.Seconds()*1000),
	)

	// Record OpenTelemetry metrics
	otellib.RecordTransactionWithService(ctx, "acquirer", respMsg.MTI, respMsg.RespCode, duration)

	c.JSON(http.StatusOK, gin.H{
		"request_hex":  hexReq,
		"response_hex": hexResp,
		"response":     respMsg,
	})
}

func (s *HTTPServer) handleTopUpCard(c *gin.Context) {
	if s.appStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
		return
	}
	id := c.Param("id")
	var body struct {
		Amount int64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}
	svc := card.NewService(s.appStore)
	resp, err := svc.TopUp(c.Request.Context(), id, body.Amount)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *HTTPServer) handleListTransactions(c *gin.Context) {
	if s.appStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
		return
	}

	limit := 50
	offset := 0
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	txs, err := s.appStore.AcquirerTransactions().ListAcquirerTransactions(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if txs == nil {
		txs = []store.AcquirerTransaction{}
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txs})
}

func (s *HTTPServer) handleListIssuerTransactions(c *gin.Context) {
	if s.appStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not configured"})
		return
	}

	limit := 50
	offset := 0
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	txs, err := s.appStore.IssuerTransactions().ListIssuerTransactions(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if txs == nil {
		txs = []store.IssuerTransaction{}
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txs})
}

// asyncLogAcquirerTx fires a non-blocking goroutine to INSERT a single acquirer
// transaction record with its final status. Optional responseCode can be provided.
func (s *HTTPServer) asyncLogAcquirerTx(req *iso.ISOMessage, hexReq *string, hexResp *string, status store.TransactionStatus, responseCode ...*string) {
	if s.appStore == nil {
		return
	}
	amount, err := parseAmount12(req.AmountTrn)
	if err != nil {
		amount = 0
	}
	var terminalID *string
	if strings.TrimSpace(req.TerminalID) != "" {
		t := strings.TrimSpace(req.TerminalID)
		terminalID = &t
	}
	var merchantID *string
	if strings.TrimSpace(req.MerchantID) != "" {
		m := strings.TrimSpace(req.MerchantID)
		merchantID = &m
	}
	var rc *string
	if len(responseCode) > 0 {
		rc = responseCode[0]
	}

	tx := &store.AcquirerTransaction{
		STAN:         strings.TrimSpace(req.STAN),
		MTI:          strings.TrimSpace(req.MTI),
		PANMasked:    maskPAN(req.PAN),
		Amount:       amount,
		CurrencyCode: strings.TrimSpace(req.CurCodeTrn),
		TerminalID:   terminalID,
		MerchantID:   merchantID,
		Status:       status,
		ResponseCode: rc,
		RequestHex:   hexReq,
		ResponseHex:  hexResp,
	}

	select {
	case s.auditCh <- tx:
	default:
	}
}

func parseAmount12(s string) (int64, error) {
	if len(s) != 12 {
		return 0, fmt.Errorf("amount must be 12 digits")
	}
	var v int64
	for i := 0; i < 12; i++ {
		b := s[i]
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("amount must be digits")
		}
		v = v*10 + int64(b-'0')
	}
	return v, nil
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
