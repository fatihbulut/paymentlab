package acquirer

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
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
	"iso-parser-service/internal/util"
)

type HTTPServer struct {
	switchInstance *AcquirerSwitch
	appStore       store.Store
	limiter        *ConcurrencyLimiter
	auditCh        chan *store.AcquirerTransaction
}

func NewHTTPServer(switchInstance *AcquirerSwitch, appStore store.Store) *HTTPServer {
	if appStore == nil {
		panic("acquirer HTTP server: store is nil - database is required")
	}
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
			"limit":          s.limiter.Limit(),
			"max_concurrent": s.limiter.MaxConcurrent(),
			"max_queue":      s.limiter.MaxQueue(),
			"rejected_total": s.limiter.RejectedTotal(),
		})
	})

	// Kubernetes-style health endpoint alias.
	router.GET("/healthz", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/health")
	})

	// Minimal Prometheus-style metrics endpoint for quick debugging.
	// Primary metrics export path is OTLP via OpenTelemetry (see internal/otel).
	router.GET("/metrics", func(c *gin.Context) {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		c.String(http.StatusOK, strings.Join([]string{
			"# HELP acquirer_in_flight_requests Current in-flight transaction requests.",
			"# TYPE acquirer_in_flight_requests gauge",
			fmt.Sprintf("acquirer_in_flight_requests %d", s.limiter.Active()),
			"# HELP acquirer_queued_requests Current queued transaction requests (waiting for slot).",
			"# TYPE acquirer_queued_requests gauge",
			fmt.Sprintf("acquirer_queued_requests %d", s.limiter.Queued()),
			"# HELP acquirer_rejected_total Total rejected transaction requests due to overload.",
			"# TYPE acquirer_rejected_total counter",
			fmt.Sprintf("acquirer_rejected_total %d", s.limiter.RejectedTotal()),
			"# HELP process_goroutines Number of goroutines.",
			"# TYPE process_goroutines gauge",
			fmt.Sprintf("process_goroutines %d", runtime.NumGoroutine()),
			"# HELP process_resident_memory_bytes Approx memory in bytes (Go runtime Sys).",
			"# TYPE process_resident_memory_bytes gauge",
			fmt.Sprintf("process_resident_memory_bytes %d", ms.Sys),
			"",
		}, "\n"))
	})

	router.POST("/v1/cards", s.handleCreateCard)
	router.GET("/v1/cards", s.handleListCards)
	router.PUT("/v1/cards/:id", s.handleUpdateCard)
	router.DELETE("/v1/cards/:id", s.handleDeleteCard)
	router.POST("/v1/cards/:id/topup", s.handleTopUpCard)
	router.POST("/v1/transaction", s.requestTimeoutMiddleware(), s.limiter.Middleware(), s.handleTransaction)
	router.GET("/v1/transactions", s.handleListTransactions)
	router.GET("/v1/issuer-transactions", s.handleListIssuerTransactions)

	return router
}

func (s *HTTPServer) requestTimeoutMiddleware() gin.HandlerFunc {
	timeoutSec := 2
	if v := os.Getenv("ACQUIRER_REQUEST_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}
	d := time.Duration(timeoutSec) * time.Second

	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *HTTPServer) handleCreateCard(c *gin.Context) {
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
	id := c.Param("id")
	svc := card.NewService(s.appStore)
	if err := svc.DeleteCard(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// handleTransaction: Ana transaction işleme endpoint'i
//
// AKIŞ:
// 1. JSON → ISO8583 message parse
// 2. ISO message → hex pack (binary format)
// 3. Hex → raw bytes decode
// 4. Switch'e gönder (TPDU wrapper ile)
// 5. Response bekle (timeout ile)
// 6. Response → hex → ISO message parse
// 7. Async audit log (non-blocking)
// 8. Timing metrics log
//
// ÖNEMLİ: Tüm adımlar timing'lenmiş (pack, switch_rtt, parse, total)
func (s *HTTPServer) handleTransaction(c *gin.Context) {
	// OpenTelemetry tracing başlat (distributed tracing için)
	tracer := otel.Tracer("acquirer-tracer")
	ctx, span := tracer.Start(c.Request.Context(), "transaction-processing")
	defer span.End()

	start := time.Now() // Toplam süre için
	var req iso.ISOMessage

	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "invalid_json"),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ISOMessage JSON"})
		return
	}

	// ISO message → hex string (binary format)
	// Örn: {MTI:"0200", PAN:"4111..."} → "0200..."
	hexReq, err := iso.PackMessageToHex(&req)
	packDone := time.Now() // Pack süresi için
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
		attribute.String("transaction.pan_masked", util.MaskPAN(req.PAN)),
	)

	// Hex string → raw bytes (TCP üzerinden göndermek için)
	// Örn: "0200..." → []byte{0x02, 0x00, ...}
	rawISOBytes, err := hex.DecodeString(hexReq)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "hex_decode_error"),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("hex decode error: %v", err)})
		return
	}

	// Switch'e gönder ve response bekle
	// ÖNEMLİ: Bu blocking call (timeout ile)
	// Switch internal olarak: TPDU wrap → TCP send → response wait → TPDU unwrap
	switchStart := time.Now()
	response, err := s.switchInstance.HandleTerminalRequest(ctx, rawISOBytes)
	switchDone := time.Now() // switch_rtt: en kritik metrik
	if err != nil {
		// Switch error: timeout, connection error, vs.
		// Async audit log (non-blocking: channel'a at, worker goroutine yazar)
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

	// Async audit log: tek INSERT, final status ile (non-blocking)
	// ÖNEMLİ: Audit log ana işlemi bloklamaz, channel'a at ve devam et
	// Worker goroutine (3 adet) background'da DB'ye yazar
	{
		status := store.TransactionStatus("DECLINED") // Default: declined
		if respMsg.RespCode == "00" {                 // "00" = approved
			status = store.TransactionStatus("APPROVED")
		}
		rc := respMsg.RespCode
		s.asyncLogAcquirerTx(&req, &hexReq, &hexResp, status, &rc)
	}

	parseDone := time.Now()

	// Timing log: her adımın süresini log'la (performance debugging için)
	// queue: limiter'da bekleme süresi (0 ise hemen slot bulunmuş)
	// pack: ISO message → hex dönüşüm süresi
	// switch_rtt: issuer'a gidip gelme süresi (EN ÖNEMLİ METRİK)
	// parse: hex → ISO message parse süresi
	// total: toplam süre (HTTP request başından sona)
	queueWait := float64(0)
	if v, exists := c.Get("queue_wait_ms"); exists {
		queueWait = v.(float64) // Middleware'den gelen değer
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
//
// ASYNC PATTERN:
// - Ana işlemi bloklamaz (channel'a at ve devam et)
// - Channel buffer: 1000 (burst traffic'i absorbe eder)
// - 3 worker goroutine: channel'dan okur ve DB'ye yazar
// - Channel full ise: drop (select default case)
//
// ÖNEMLİ: Audit log başarısız olsa bile transaction devam eder
// (audit log transaction'ı bloklamaz)
func (s *HTTPServer) asyncLogAcquirerTx(req *iso.ISOMessage, hexReq *string, hexResp *string, status store.TransactionStatus, responseCode ...*string) {
	amount, err := util.ParseAmount12(req.AmountTrn)
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
		PANMasked:    util.MaskPAN(req.PAN),
		Amount:       amount,
		CurrencyCode: strings.TrimSpace(req.CurCodeTrn),
		TerminalID:   terminalID,
		MerchantID:   merchantID,
		Status:       status,
		ResponseCode: rc,
		RequestHex:   hexReq,
		ResponseHex:  hexResp,
	}

	// Channel'a gönder (non-blocking)
	select {
	case s.auditCh <- tx:
		// Başarılı: worker goroutine alacak ve DB'ye yazacak
	default:
		// Channel full: drop (audit log kaybı kabul edilebilir)
		// ÖNEMLİ: Ana işlemi bloklamak yerine audit log'u drop et
	}
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
