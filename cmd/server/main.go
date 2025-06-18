package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"servicesyncdata/config"
	"servicesyncdata/internal/handler"
)

func main() {
	// Menghubungkan ke database MySQL
	config.ConnectDB()

	// Menginisialisasi router Gin
	r := gin.Default()

	// Menyiapkan route
	r.POST("/produce", handler.ProduceMessageHandler)
	r.GET("/consume", handler.ConsumeMessagesHandler)

	// Menjalankan server
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
