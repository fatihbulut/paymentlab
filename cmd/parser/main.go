package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/moov-io/iso8583"
)

// Gin'in otomatik JSON bağlaması için struct tag ekliyoruz
type ParseRequest struct {
	RawHex string `json:"raw_hex" binding:"required"`
}

func main() {

	// Gin modunu ayarla (Render'da release mode daha iyidir)
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// 1. Sağlık Kontrolü (Health Check)
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "Working!")
	})

	// 2. Ana Parse Endpoint'i
	router.POST("/v1/parse", func(c *gin.Context) {
		var req ParseRequest

		// JSON'u otomatik bind et (Hata varsa direkt 400 döner)
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[HATA] JSON Bind Hatası: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz JSON veya eksik raw_hex"})
			return
		}

		log.Printf("Gelen ISO Mesajı: %s", req.RawHex)

		// Hex -> Byte
		rawBytes, err := hex.DecodeString(req.RawHex)
		if err != nil {
			log.Printf("[HATA] Hex Decode Hatası: %v | Veri: %s", err, req.RawHex)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Hex formatı hatalı"})
			return
		}

		// ISO Unpack (isoFields.go'daki MessageSpec'i kullanır)
		message := iso8583.NewMessage(spec)
		if err := message.Unpack(rawBytes); err != nil {
			log.Printf("[HATA] ISO Unpack Hatası: %v | Veri: %s", err, req.RawHex)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO Parse Hatası: %v", err)})
			return
		}

		// Struct'a aktar (isoModel.go'daki MyMessage'ı kullanır)
		msgData := &MyMessage{}
		message.Unmarshal(msgData)

		// Başarılı Yanıt
		c.JSON(http.StatusOK, msgData)
	})

	// Render port ayarı
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf(" Gin Sunucusu %s portunda hazır!\n", port)
	router.Run(":" + port)

}
