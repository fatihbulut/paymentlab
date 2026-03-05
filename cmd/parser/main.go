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

type ParseRequest struct {
	RawHex string `json:"raw_hex" binding:"required"`
}

func main() {

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.POST("/v1/parse", func(c *gin.Context) {
		var req ParseRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("[HATA] JSON Bind Hatası: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz JSON veya eksik raw_hex"})
			return
		}

		log.Printf("Gelen ISO Mesajı: %s", req.RawHex)

		rawBytes, err := hex.DecodeString(req.RawHex)
		if err != nil {
			log.Printf("[HATA] Hex Decode Hatası: %v | Veri: %s", err, req.RawHex)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Hex formatı hatalı"})
			return
		}

		message := iso8583.NewMessage(spec)
		if err := message.Unpack(rawBytes); err != nil {
			log.Printf("[HATA] ISO Unpack Hatası: %v | Veri: %s", err, req.RawHex)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO Parse Hatası: %v", err)})
			return
		}

		msgData := &ISOMessage{}
		message.Unmarshal(msgData)

		c.JSON(http.StatusOK, msgData)
	})

	router.POST("/v1/pack", func(c *gin.Context) {
		var incomingData ISOMessage

		if err := c.ShouldBindJSON(&incomingData); err != nil {
			log.Printf("[HATA] JSON Bind Hatası: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz veri formatı"})
			return
		}

		message := iso8583.NewMessage(spec)
		message.MTI(incomingData.MTI)
		if err := message.Marshal(&incomingData); err != nil {
			log.Printf("[HATA] ISO Marshal Hatası: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ISO paketleme hazırlığı başarısız"})
			return
		}

		rawBytes, err := message.Pack()
		if err != nil {
			log.Printf("[HATA] ISO Pack Hatası: %v", err)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO Pack Hatası: %v", err)})

		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "Success",
			"hex":     hex.EncodeToString(rawBytes),
			"length":  len(rawBytes),
			"details": incomingData,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Gin Sunucusu %s portunda hazır!\n", port)
	router.Run(":" + port)

}
