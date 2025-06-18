package service

import (
	"log"
	"servicesyncdata/internal/repository"
	"strconv"
)

// ProduceMessage mengirim pesan ke Kafka
func ProduceMessage(name string, age int, city string) error {
	// Buat pesan dalam format JSON
	message := `{"name": "` + name + `", "age": ` + strconv.Itoa(age) + `, "city": "` + city + `"}`

	// Inisialisasi Kafka repository
	kafkaRepo := repository.NewKafkaRepository()

	// Mengirim pesan ke Kafka
	err := kafkaRepo.WriteMessage(message)
	if err != nil {
		log.Println("Error sending message:", err)
		return err
	}

	return nil
}

// ConsumeMessages membaca pesan dari Kafka
func ConsumeMessages() {
	kafkaRepo := repository.NewKafkaRepository()

	for {
		// Membaca pesan dari Kafka
		msg, err := kafkaRepo.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		// Proses pesan yang diterima
		log.Println("Received message:", msg)
	}
}
