package repository

import (
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
)

// KafkaRepository adalah struktur yang berisi Kafka writer dan reader
type KafkaRepository struct {
	Writer *kafka.Writer
	Reader *kafka.Reader
}

func NewKafkaRepository() *KafkaRepository {
	// Inisialisasi Kafka writer
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "people_topic",
		Balancer: &kafka.LeastBytes{},
	}

	// Inisialisasi Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "people_topic",
		GroupID:  "group_people",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	log.Println("KafkaReader created successfully")

	return &KafkaRepository{
		Writer: writer,
		Reader: reader,
	}
}

// WriteMessage untuk mengirimkan pesan ke Kafka
func (r *KafkaRepository) WriteMessage(message string) error {
	err := r.Writer.WriteMessages(nil, kafka.Message{
		Value: []byte(message),
	})
	if err != nil {
		log.Println("Error sending message to Kafka:", err)
		return err
	}
	return nil
}

// ReadMessage untuk membaca pesan dari Kafka
func (r *KafkaRepository) ReadMessage() (string, error) {
	if r == nil || r.Reader == nil {
		return "", fmt.Errorf("Kafka reader not initialized")
	}

	msg, err := r.Reader.ReadMessage(nil)
	if err != nil {
		log.Println("Error reading message from Kafka:", err)
		return "", err
	}
	log.Printf("Received message: %s", msg.Value)
	return string(msg.Value), nil
}
