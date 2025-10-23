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
	kafkaBroker = "localhost:9092"
)

type KafkaMessage struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Age       int       `json:"age"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type KafkaMessageForInsert struct {
	Header string `json:"header"`
	JSON   string `json:"json"`
}

// ========================================================
// === FUNGSI UNTUK DATABASE ORACLE
// ========================================================

func insertIntoOracle(conn *sql.DB, message KafkaMessageForInsert, offset int64) error {
	query := `INSERT INTO testkafka2 (header, json, category, created_at, updated_at) 
	          VALUES (:header, :json, 'Kafka', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	_, err := conn.Exec(query,
		sql.Named("header", fmt.Sprintf("Offset: %d", offset)),
		sql.Named("json", message.JSON),
	)
	return err
}

func fetchDataFromDatabase(conn *sql.DB) ([]KafkaMessage, error) {
	var messages []KafkaMessage
	rows, err := conn.Query(`SELECT ID, NAME, AGE, EMAIL, CREATED_AT, UPDATED_AT 
		FROM testkafka 
		WHERE CREATED_AT >= CURRENT_TIMESTAMP - INTERVAL '12' HOUR AND NAME = 'JOKO'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var message KafkaMessage
		if err := rows.Scan(&message.ID, &message.Name, &message.Age, &message.Email, &message.CreatedAt, &message.UpdatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

// ========================================================
// === FUNGSI UNTUK MEMBUAT TOPIC KAFKA SECARA MANUAL
// ========================================================

func createKafkaTopic(topic string, partitions int, replicationFactor int) error {
	conn, err := kafka.Dial("tcp", kafkaBroker)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	// Cek apakah topic sudah ada
	partitionsList, err := conn.ReadPartitions()
	if err != nil {
		return fmt.Errorf("failed to read partitions: %v", err)
	}
	for _, p := range partitionsList {
		if p.Topic == topic {
			log.Printf("⚠️  Topic '%s' sudah ada — tidak perlu dibuat ulang\n", topic)
			return nil
		}
	}

	// Ambil controller untuk membuat topic
	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %v", err)
	}
	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %v", err)
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     partitions,
			ReplicationFactor: replicationFactor,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("failed to create topic '%s': %v", topic, err)
	}

	log.Printf("🎉 Topic '%s' berhasil dibuat dengan %d partition dan replication factor %d\n",
		topic, partitions, replicationFactor)
	return nil
}

// ========================================================
// === MAIN PROGRAM
// ========================================================

func main() {
	if len(os.Args) < 3 {
		fmt.Println(`
Usage:
  go run main.go create producer <topic>         -> Buat topic Kafka
  go run main.go producer <topic>                -> Jalankan producer
  go run main.go consumer <topic> <group_id>     -> Jalankan consumer
`)
		return
	}

	command := os.Args[1]
	mode := os.Args[2]
	var topic string

	if command == "create" {
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run main.go create producer <topic>")
			return
		}
		topic = os.Args[3]
	} else {
		topic = os.Args[2]
	}

	// Context untuk Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	// Koneksi ke Oracle (hanya dibutuhkan untuk producer dan consumer)
	connStr := go_ora.BuildUrl("10.8.2.219", 1521, "JNEBILL", "JNEBILL", "JNE98292092B5494083OK", nil)
	dbConn, err := sql.Open("oracle", connStr)
	if err != nil {
		log.Fatal("Koneksi gagal:", err)
	}
	defer dbConn.Close()

	switch command {
	// ======================================================
	// === CREATE TOPIC
	// ======================================================
	case "create":
		if mode != "producer" {
			fmt.Println("Usage: go run main.go create producer <topic>")
			return
		}
		if err := createKafkaTopic(topic, 1, 1); err != nil {
			log.Fatal("Gagal membuat topic:", err)
		}
		log.Printf("✅ Topic '%s' siap digunakan\n", topic)

	// ======================================================
	// === PRODUCER
	// ======================================================
	case "producer":
		writer := &kafka.Writer{
			Addr:         kafka.TCP(kafkaBroker),
			Async:        true,
			RequiredAcks: kafka.RequireOne,
			BatchSize:    1000,
			BatchTimeout: time.Second,
			Compression:  kafka.Gzip,
		}
		defer writer.Close()

		var messageID int
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("⛔ Producer stopped by signal.")
				return
			case <-ticker.C:
				data, err := fetchDataFromDatabase(dbConn)
				if err != nil {
					log.Println("Error fetching data:", err)
					continue
				}
				for _, message := range data {
					messageJSON, err := json.Marshal(message)
					if err != nil {
						log.Println("Error marshaling:", err)
						continue
					}
					key := fmt.Sprintf("%d-%s", messageID, message.Name)
					err = writer.WriteMessages(ctx, kafka.Message{
						Topic: topic,
						Key:   []byte(key),
						Value: messageJSON,
					})
					if err != nil {
						log.Println("Error sending message:", err)
						continue
					}
					log.Printf("📤 Sent message with key %s to topic %s", key, topic)
					messageID++
				}
			}
		}

	// ======================================================
	// === CONSUMER
	// ======================================================
	case "consumer":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run main.go consumer <topic> <group_id>")
			return
		}
		groupId := os.Args[3]

		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  []string{kafkaBroker},
			Topic:    topic,
			GroupID:  groupId,
			MaxBytes: 10e6,
		})
		defer reader.Close()

		for {
			m, err := reader.FetchMessage(ctx)
			if errors.Is(err, context.Canceled) {
				log.Println("⛔ Consumer stopped by signal.")
				break
			}
			if err != nil {
				log.Println("Error fetching message:", err)
				continue
			}

			var message KafkaMessage
			if err := json.Unmarshal(m.Value, &message); err != nil {
				log.Println("Error unmarshalling:", err)
				continue
			}

			log.Printf("📥 Consumed message Key=%s, Topic=%s", string(m.Key), topic)

			kafkaMessageForInsert := KafkaMessageForInsert{
				Header: fmt.Sprintf("Header for %s", message.Name),
				JSON:   string(m.Value),
			}

			if err := insertIntoOracle(dbConn, kafkaMessageForInsert, m.Offset); err != nil {
				log.Println("Error inserting to Oracle:", err)
				continue
			}

			log.Printf("✅ Data for %s inserted to Oracle DB", message.Name)

			if err := reader.CommitMessages(ctx, m); err != nil {
				log.Println("Error committing:", err)
			}
		}

	default:
		fmt.Println("Perintah tidak dikenal. Gunakan: create | producer | consumer")
	}
}
