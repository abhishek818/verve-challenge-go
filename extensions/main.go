package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

var (
	ctx         = context.Background()
	redisDB     *redis.Client
	kafkaWriter *kafka.Writer
)

func initRedis() *redis.Client {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort,
		Password: "",
		DB:       0,
	})

	// Test connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	return rdb
}

func initKafka() *kafka.Writer {
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	kafkaTopic := os.Getenv("KAFKA_TOPIC")

	writer := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    kafkaTopic,
		Balancer: &kafka.LeastBytes{},
	}

	return writer
}

func waitForKafka(broker string, retries int, delay time.Duration) error {
	for i := 0; i < retries; i++ {
		conn, err := kafka.Dial("tcp", broker)
		if err == nil {
			// Successfully connected to Kafka
			conn.Close()
			log.Printf("Connected to Kafka at %s", broker)
			return nil
		}

		log.Printf("Failed to connect to Kafka at %s: %v. Retrying in %v...", broker, err, delay)
		time.Sleep(delay)
	}
	return fmt.Errorf("could not connect to Kafka at %s after %d retries", broker, retries)
}

func createKafkaTopic(topic string, broker string) {
	err := waitForKafka(broker, 10, 5*time.Second)
	if err != nil {
		log.Fatalf("Kafka is not ready: %v", err)
	}

	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		log.Fatalf("Failed to connect to Kafka broker: %v", err)
	}
	defer conn.Close()

	// Create topic
	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	if err != nil {
		log.Printf("Failed to create Kafka topic %s: %v", topic, err)
	} else {
		log.Printf("Kafka topic %s created successfully", topic)
	}
}

// Publish unique ID count to Kafka
func publishToKafka(count int) {
	payload := map[string]interface{}{
		"unique_request_count": count,
		"timestamp":            time.Now().Format(time.RFC3339),
	}
	message, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal Kafka message: %v\n", err)
		return
	}

	// Write message to Kafka
	err = kafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte("unique-id-count"),
		Value: message,
	})
	if err != nil {
		log.Printf("Failed to write message to Kafka: %v\n", err)
	} else {
		log.Printf("Published to Kafka: %s\n", string(message))
	}
}

// Periodically fetch unique ID counts and send to Kafka
func logAndNotifyUniqueRequests() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Count unique requests in Redis
		keys, err := redisDB.Keys(ctx, "*").Result()
		if err != nil {
			log.Printf("Error fetching keys from Redis: %v\n", err)
			continue
		}

		count := len(keys)

		publishToKafka(count)

		// Clean up old keys
		for _, key := range keys {
			redisDB.Del(ctx, key)
		}
	}
}

// Send unique request count to an endpoint
func sendCountToEndpoint(endpoint string, count int) {
	payload := map[string]interface{}{
		"unique_request_count": count,
		"timestamp":            time.Now().Format(time.RFC3339),
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal JSON payload: %v\n", err)
		return
	}

	// Send the POST request
	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending request to endpoint %s: %v\n", endpoint, err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Sent count to endpoint %s, status code: %d\n", endpoint, resp.StatusCode)
}

func isUniqueID(id int) bool {
	key := strconv.Itoa(id)
	result, err := redisDB.SetNX(ctx, key, true, 1*time.Minute).Result()
	if err != nil {
		log.Printf("Error checking ID in Redis: %v\n", err)
		return false
	}
	return result
}

func acceptHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure it's a GET request
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	idParam := query.Get("id")
	endpoint := query.Get("endpoint")

	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid or missing 'id' parameter", http.StatusBadRequest)
		return
	}

	if !isUniqueID(id) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok (duplicate), retry with different id"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))

	if endpoint != "" {
		keys, _ := redisDB.Keys(ctx, "*").Result()
		count := len(keys)

		go sendCountToEndpoint(endpoint, count)
	}
}

func main() {
	redisDB = initRedis()
	defer redisDB.Close()

	kafkaWriter = initKafka()
	defer kafkaWriter.Close()

	createKafkaTopic("unique-id-count", os.Getenv("KAFKA_BROKER"))

	go logAndNotifyUniqueRequests()

	http.HandleFunc("/api/verve/accept", acceptHandler)

	// Start the server
	port := ":8080"
	log.Printf("Starting server on %s...\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
