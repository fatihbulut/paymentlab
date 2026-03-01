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

	router.GET("/v1/test-pack-unpack", func(c *gin.Context) {

		// 1. Az önce yazdığımız fonksiyonu ÇAĞIR (Generator)
		rawBytes, err := CreateAuthRequest(spec, "1234567891011111", 1000, "123456")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Pack Hatası", "details": err.Error()})
			return
		}

		// 2. Kendi ürettiğimizi PARSE ET (Unpack)
		incomingMsg := iso8583.NewMessage(spec)
		if err := incomingMsg.Unpack(rawBytes); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Unpack Hatası", "hex": hex.EncodeToString(rawBytes)})
			return
		}

		// 3. Modele Dök (Unmarshal)
		msgData := &MyMessage{}
		incomingMsg.Unmarshal(msgData)

		// 4. Sonucu Dön
		c.JSON(http.StatusOK, gin.H{
			"message": "Round-trip successful!",
			"hex":     hex.EncodeToString(rawBytes),
			"parsed":  msgData,
		})
	})

	// 4. JSON -> ISO (Hex) Pack Endpoint'i
	router.POST("/v1/pack", func(c *gin.Context) {
		var incomingData MyMessage

		// 1. Gelen JSON'u MyMessage struct'ına bağla
		if err := c.ShouldBindJSON(&incomingData); err != nil {
			log.Printf("[HATA] JSON Bind Hatası: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Geçersiz veri formatı"})
			return
		}

		// 2. Yeni bir ISO mesajı oluştur
		message := iso8583.NewMessage(spec)
		message.MTI(incomingData.MTI)
		// 3. Struct'taki verileri ISO mesajına "Marshal" et (Ters işlem)
		// moov-io kütüphanesi struct tag'lerine bakarak alanları otomatik doldurur
		if err := message.Marshal(&incomingData); err != nil {
			log.Printf("[HATA] ISO Marshal Hatası: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ISO paketleme hazırlığı başarısız"})
			return
		}

		// 4. Binary/Hex haline getir (Pack)
		rawBytes, err := message.Pack()
		if err != nil {
			log.Printf("[HATA] ISO Pack Hatası: %v", err)
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("ISO Pack Hatası: %v", err)})

		}

		// 5. Yanıtı dön
		c.JSON(http.StatusOK, gin.H{
			"status":  "Success",
			"hex":     hex.EncodeToString(rawBytes),
			"length":  len(rawBytes),
			"details": incomingData, // Gönderdiğin veriyi teyit için geri dönüyoruz
		})
	})

	// Render port ayarı
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf(" Gin Sunucusu %s portunda hazır!\n", port)
	router.Run(":" + port)

}
