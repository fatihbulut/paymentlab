package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"iso-parser-service/internal/iso"
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

		msgData, err := iso.ParseHexToMessage(req.RawHex)
		if err != nil {
			log.Printf("[HATA] ISO Unpack Hatası: %v | Veri: %s", err, req.RawHex)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO Parse Hatası: %v", err)})
			return
		}

		c.JSON(http.StatusOK, msgData)
	})

	router.POST("/v1/pack", func(c *gin.Context) {
		var incomingData iso.ISOMessage

		if err := c.ShouldBindJSON(&incomingData); err != nil {
			log.Printf("[HATA] JSON Bind Hatası: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz veri formatı"})
			return
		}

		hexStr, err := iso.PackMessageToHex(&incomingData)
		if err != nil {
			log.Printf("[HATA] ISO Pack Hatası: %v", err)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO Pack Hatası: %v", err)})
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "Success",
			"hex":     hexStr,
			"length":  len(hexStr) / 2,
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
