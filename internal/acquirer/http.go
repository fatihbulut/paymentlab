package acquirer

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

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
	router := gin.Default()

	router.POST("/v1/transaction", s.handleTransaction)

	return router
}

func (s *HTTPServer) handleTransaction(c *gin.Context) {
	var req iso.ISOMessage

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("acquirer: JSON bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "geçersiz ISOMessage JSON"})
		return
	}

	hexReq, err := iso.PackMessageToHex(&req)
	if err != nil {
		log.Printf("acquirer: pack request error: %v", err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO pack hatası: %v", err)})
		return
	}

	hexResp, err := s.issuerClient.SendAndReceive(hexReq)
	if err != nil {
		log.Printf("acquirer: send to issuer error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer ile iletişim hatası: %v", err)})
		return
	}

	respMsg, err := iso.ParseHexToMessage(hexResp)
	if err != nil {
		log.Printf("acquirer: parse issuer response error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("issuer yanıtı parse hatası: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_hex":  hexReq,
		"response_hex": hexResp,
		"response":     respMsg,
	})
}
