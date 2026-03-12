package acquirer

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"iso-parser-service/internal/iso"
	otelmetrics "iso-parser-service/internal/otel"
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
	router.Use(otelgin.Middleware("acquirer"))

	// Serve UI
	router.GET("/", func(c *gin.Context) {
		c.File("web/index.html")
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/v1/transaction", s.handleTransaction)

	return router
}

func (s *HTTPServer) handleTransaction(c *gin.Context) {
	start := time.Now()
	var req iso.ISOMessage

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz ISOMessage JSON"})
		return
	}

	hexReq, err := iso.PackMessageToHex(&req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO pack hatası: %v", err)})
		return
	}

	hexResp, err := s.issuerClient.SendAndReceive(c.Request.Context(), hexReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer ile iletişim hatası: %v", err)})
		return
	}

	respMsg, err := iso.ParseHexToMessage(hexResp)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer yanıtı parse hatası: %v", err)})
		return
	}

	// Record metrics
	duration := time.Since(start)
	otelmetrics.RecordTransactionWithService(c.Request.Context(), "acquirer", respMsg.MTI, respMsg.RespCode, duration)

	c.JSON(http.StatusOK, gin.H{
		"request_hex":  hexReq,
		"response_hex": hexResp,
		"response":     respMsg,
	})
}
