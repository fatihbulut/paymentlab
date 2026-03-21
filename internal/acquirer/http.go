package acquirer

import (
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
	issuerClient IssuerClient
}

func NewHTTPServer(client IssuerClient) *HTTPServer {
	return &HTTPServer{issuerClient: client}
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
		c.File("web/og-image.png")
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/v1/transaction", s.handleTransaction)

	return router
}

func (s *HTTPServer) handleTransaction(c *gin.Context) {
	// Root Trace: HTTP Request ile başlar
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz ISOMessage JSON"})
		return
	}

	hexReq, err := iso.PackMessageToHex(&req)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "iso_pack_error"),
		)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO pack hatası: %v", err)})
		return
	}

	// Context Propagation: Trace bilgisini TCP isteğine ekle
	span.SetAttributes(
		attribute.String("transaction.mti", req.MTI),
		attribute.String("transaction.pan_masked", maskPAN(req.PAN)),
	)

	hexResp, err := s.issuerClient.SendAndReceive(ctx, hexReq)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "issuer_communication_error"),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer ile iletişim hatası: %v", err)})
		return
	}

	respMsg, err := iso.ParseHexToMessage(hexResp)
	if err != nil {
		span.SetAttributes(
			attribute.String("transaction.status", "failed"),
			attribute.String("error", "response_parse_error"),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer yanıtı parse hatası: %v", err)})
		return
	}

	// Record metrics
	duration := time.Since(start)
	span.SetAttributes(
		attribute.String("transaction.status", "completed"),
		attribute.String("transaction.response_code", respMsg.RespCode),
		attribute.Float64("transaction.duration_ms", duration.Seconds()*1000),
	)

	// Sadece kritik sağlık metrikleri
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

// recordHealthMetrics records only critical health metrics
func recordHealthMetrics() {
	// Aktif İş Parçacığı (Goroutine) Sayısı
	var goroutines = runtime.NumGoroutine()

	// Bellek Kullanımı (RSS) - Linux/Unix sistemleri için
	var rss int64
	// Basit RSS ölçümü - production'da daha gelişmiş yöntem kullanılabilir
	rss = estimateRSSMemory()

	fmt.Printf("HEALTH_METRIC: service=acquirer goroutines=%d rss_bytes=%d\n", goroutines, rss)
}

// estimateRSSMemory basit RSS tahmini yapar
func estimateRSSMemory() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// Sys değerinden heap tahmini çıkararak basit RSS tahmini
	return int64(m.Sys) - int64(m.HeapSys)
}
