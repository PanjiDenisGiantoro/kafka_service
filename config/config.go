package config

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/segmentio/kafka-go"
	"log"
)

// Kafka configuration
var KafkaBrokers = []string{"localhost:9092"} // Ganti dengan alamat Kafka Anda
var KafkaTopic = "people_topic"

// MySQL configuration
var DB *gorm.DB

// ConnectDB initializes MySQL connection
func ConnectDB() {
	var err error
	DB, err = gorm.Open("mysql", "root@tcp(localhost:3306)/kafka?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
}

// KafkaWriter initializes Kafka writer
func KafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP(KafkaBrokers...),
		Topic:    KafkaTopic,
		Balancer: &kafka.LeastBytes{},
	}
}
