package handler

import (
	"github.com/gin-gonic/gin"
	"servicesyncdata/internal/service"
)

// Handler untuk mengirimkan data ke Kafka
func ProduceMessageHandler(c *gin.Context) {
	var request struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		City string `json:"city"`
	}

	// Parsing JSON dari body request
	if err := c.BindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// Mengirimkan data ke service untuk diproses
	err := service.ProduceMessage(request.Name, request.Age, request.City)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to produce message"})
		return
	}

	c.JSON(200, gin.H{"status": "Message sent to Kafka successfully"})
}

// Handler untuk menerima data dari Kafka
func ConsumeMessagesHandler(c *gin.Context) {
	// Menjalankan consumer untuk membaca pesan dari Kafka secara asynchronous
	go service.ConsumeMessages()

	c.JSON(200, gin.H{"status": "Kafka consumer started"})
}
