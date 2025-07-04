package main

import (
	"bufio"
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
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	topicName   = "mytopic"
	kafkaBroker = "localhost:9092"
	port        = 1522
)

type KafkaMessage struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

type KafkaMessageForInsert struct {
	Header string `json:"header"`
	JSON   string `json:"json"`
}

// Fungsi untuk insert data ke Oracle menggunakan go-ora
func insertIntoOracle(conn *sql.DB, message KafkaMessageForInsert) error {
	// Gunakan sql.Named dengan benar untuk mengikat nilai ke query
	query := `INSERT INTO testkafka2 (header, json, created_at, updated_at) 
	          VALUES (:header, :json, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	_, err := conn.Exec(query, sql.Named("header", message.Header), sql.Named("json", message.JSON))
	return err
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
	connStr := go_ora.BuildUrl("10.18.2.19", port, "TRAINING", "JNEBILL", "JNEBILL", nil)
	conn, err := sql.Open("oracle", connStr)
	if err != nil {
		log.Fatal("Koneksi gagal:", err)
	}
	defer conn.Close()

	switch os.Args[1] {
	case "producer":
		writer := &kafka.Writer{
			Addr:         kafka.TCP(kafkaBroker),
			Async:        true,
			RequiredAcks: kafka.RequireOne,
			BatchSize:    100,
			BatchTimeout: time.Second,
			Compression:  kafka.Gzip,
		}
		defer writer.Close()

		var messageID int // Initialize a counter to simulate offset ID for Key

		// Mengirim pesan ke Kafka
		for {
			fmt.Print("> ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			response = strings.TrimSuffix(response, "\n")

			if response == "" {
				break
			}

			// Parse input "Jane Doe, 28, janedoe@example.com"
			parts := strings.Split(response, ",")
			if len(parts) != 3 {
				fmt.Println("Input format salah. Format yang benar: Name, Age, Email")
				continue
			}

			// Trim space and create KafkaMessage
			name := strings.TrimSpace(parts[0])
			age, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				fmt.Println("Usia tidak valid:", parts[1])
				continue
			}
			email := strings.TrimSpace(parts[2])

			// Membuat objek KafkaMessage
			message := KafkaMessage{
				Name:  name,
				Age:   age,
				Email: email,
			}

			// Convert KafkaMessage menjadi JSON
			messageJSON, err := json.Marshal(message)
			if err != nil {
				fmt.Println("Error marshaling message:", err)
				continue
			}

			// Create Key (using a combination of offset and name)
			key := fmt.Sprintf("%d-%s", messageID, name)

			// Kirim pesan ke Kafka dengan Key
			fmt.Println("Sending message:", string(messageJSON), "with Key:", key)
			if err := writer.WriteMessages(ctx, kafka.Message{
				Topic: topicName,
				Value: messageJSON,
				Key:   []byte(key), // Set Key as offset and name
			}); err != nil {
				if errors.Is(err, context.Canceled) {
					break
				}
				panic(err)
			}

			messageID++ // Increment the message ID (offset)
		}
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
				fmt.Println("Error fetching message:", err)
				continue
			}

			// Parse JSON yang diterima dari Kafka
			var message KafkaMessage
			if err := json.Unmarshal(m.Value, &message); err != nil {
				fmt.Println("Error unmarshalling message:", err, "Message:", string(m.Value))
				continue
			}

			// Log Key yang berisi offset ID dan name
			fmt.Printf("Consumed message with Key: %s\n", string(m.Key))

			// Siapkan data untuk insert ke Oracle
			kafkaMessageForInsert := KafkaMessageForInsert{
				Header: fmt.Sprintf("Header for %s", message.Name),
				JSON:   string(m.Value),
			}

			// Insert data ke Oracle DB
			if err := insertIntoOracle(conn, kafkaMessageForInsert); err != nil {
				fmt.Println("Error inserting data to Oracle:", err)
				continue
			}

			// Log untuk menandakan bahwa data telah berhasil disimpan ke database
			fmt.Printf("Data for %s (ID: %d) successfully inserted into Oracle DB\n", message.Name, message.Age)

			// Commit pesan Kafka setelah sukses insert
			err = consumer.CommitMessages(ctx, m)
			if err != nil {
				fmt.Println("Error committing message:", err)
				continue
			}
		}
	}
}
