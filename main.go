package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/sijms/go-ora/v2"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	topicName   = "mytopic"
	kafkaBroker = "localhost:9092"
)

type KafkaMessage struct {
	ID        int       `json:"id"` // Menyimpan ID
	Name      string    `json:"name"`
	Age       int       `json:"age"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"` // Menyimpan waktu pembuatan
	UpdatedAt time.Time `json:"updated_at"` // Menyimpan waktu pembaruan
}
type KafkaMessageForInsert struct {
	Header string `json:"header"`
	JSON   string `json:"json"`
}

// Fungsi untuk insert data ke Oracle menggunakan go-ora
func insertIntoOracle(conn *sql.DB, message KafkaMessageForInsert, offset int64) error {
	// Update query SQL untuk memasukkan offset ke dalam kolom header
	query := `INSERT INTO testkafka2 (header, json, category, created_at, updated_at) 
	          VALUES (:header, :json, 'Kafka', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	_, err := conn.Exec(query, sql.Named("header", fmt.Sprintf("Offset: %d", offset)), sql.Named("json", message.JSON))
	return err
}

func fetchDataFromDatabase(conn *sql.DB) ([]KafkaMessage, error) {
	var messages []KafkaMessage
	rows, err := conn.Query(`SELECT ID, NAME, AGE, EMAIL, CREATED_AT, UPDATED_AT FROM testkafka WHERE CREATED_AT >= CURRENT_TIMESTAMP - INTERVAL '12' HOUR and NAME ='JOKO' 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var message KafkaMessage
		// Update rows.Scan() untuk menangani 6 kolom
		if err := rows.Scan(&message.ID, &message.Name, &message.Age, &message.Email, &message.CreatedAt, &message.UpdatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}
func main() {
	// Membuat konteks untuk menangani signal interupsi (CTRL+C)
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	// Koneksi ke Oracle menggunakan go-ora
	connStr := go_ora.BuildUrl("10.8.2.219", 1521, "JNEBILL", "JNEBILL", "JNE98292092B5494083OK", nil)
	conn, err := sql.Open("oracle", connStr)
	if err != nil {
		log.Fatal("Koneksi gagal:", err)
	}
	defer conn.Close()

	// Menangani argumen untuk menjalankan producer atau consumer
	switch os.Args[1] {
	// Fungsi Producer untuk mengirimkan data ke Kafka
	case "producer":
		writer := &kafka.Writer{
			Addr:         kafka.TCP(kafkaBroker),
			Async:        true,
			RequiredAcks: kafka.RequireOne,
			BatchSize:    1000000,
			BatchTimeout: time.Second,
			Compression:  kafka.Gzip,
		}
		defer writer.Close()

		var messageID int // Initialize a counter to simulate offset ID for Key

		// Modifikasi scheduler menjadi interval 5 detik untuk pengujian
		ticker := time.NewTicker(10 * time.Second) // Ubah ke 5 detik
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Mengambil data dari database
				data, err := fetchDataFromDatabase(conn)
				if err != nil {
					log.Println("Error fetching data from database:", err)
					continue
				}

				// Mengirim pesan ke Kafka
				for _, message := range data {
					// Convert KafkaMessage menjadi JSON
					messageJSON, err := json.Marshal(message)
					if err != nil {
						log.Println("Error marshaling message:", err)
						continue
					}

					// Create Key (using a combination of offset and name)
					key := fmt.Sprintf("%d-%s", messageID, message.Name)

					// Kirim pesan ke Kafka dengan Key
					if err := writer.WriteMessages(ctx, kafka.Message{
						Topic: topicName,
						Value: messageJSON,
						Key:   []byte(key), // Set Key as offset and name
					}); err != nil {
						log.Println("Error sending message:", err)
						continue
					}

					messageID++ // Increment the message ID (offset)
				}
			}
		}

		// Fungsi Consumer untuk menerima pesan dari Kafka dan menyimpan ke Oracle
		// Fungsi Consumer untuk menerima pesan dari Kafka dan menyimpan ke Oracle
	case "consumer":
		if len(os.Args) < 3 {
			fmt.Println("Consumer group ID required. Example: go run main.go consumer <group_id>")
			return
		}

		groupId := os.Args[2]
		consumer := kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{kafkaBroker},
			Topic:   topicName,
			GroupID: groupId,
			// Set maximum number of messages to read at once
			MaxBytes: 10e6, // 10MB max message size
			// Optionally, set the StartOffset if you need to consume from a specific point
			StartOffset: kafka.FirstOffset,
		})

		// Membaca pesan dari Kafka dan insert ke Oracle
		for {
			m, err := consumer.FetchMessage(ctx)
			if errors.Is(err, context.Canceled) {
				break
			}
			if err != nil {
				log.Println("Error fetching message:", err)
				continue
			}

			// Parse JSON yang diterima dari Kafka
			var message KafkaMessage
			if err := json.Unmarshal(m.Value, &message); err != nil {
				log.Println("Error unmarshalling message:", err, "Message:", string(m.Value))
				continue
			}

			// Log Key yang berisi offset ID dan name
			log.Printf("Consumed message with Key: %s\n", string(m.Key))

			// Siapkan data untuk insert ke Oracle
			kafkaMessageForInsert := KafkaMessageForInsert{
				Header: fmt.Sprintf("Header for %s", message.Name),
				JSON:   string(m.Value),
			}

			// Insert data ke Oracle DB dengan menyertakan offset
			if err := insertIntoOracle(conn, kafkaMessageForInsert, m.Offset); err != nil {
				log.Println("Error inserting data to Oracle:", err)
				continue
			}

			// Log untuk menandakan bahwa data telah berhasil disimpan ke database
			log.Printf("Data for %s (ID: %d) successfully inserted into Oracle DB\n", message.Name, message.Age)

			// Commit pesan Kafka setelah sukses insert
			err = consumer.CommitMessages(ctx, m)
			if err != nil {
				log.Println("Error committing message:", err)
				continue
			}
		}

	}
}
