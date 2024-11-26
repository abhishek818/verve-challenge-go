package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	uniqueRequests sync.Map
	mu             sync.Mutex
)

func initLogger() *log.Logger {
	file, err := os.OpenFile("request_logs.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	return log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func logAndNotifyUniqueRequests(logger *log.Logger) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		count := 0
		uniqueRequests.Range(func(_, _ interface{}) bool {
			count++
			return true
		})

		uniqueRequests = sync.Map{}
		mu.Unlock()

		logger.Printf("Unique requests in the last minute: %d\n", count)
	}
}

func sendCountToEndpoint(endpoint string, count int, logger *log.Logger) {
	u, err := url.Parse(endpoint)
	if err != nil {
		logger.Printf("invalid endpoint: %v\n", err)
		return
	}

	query := u.Query()
	query.Set("count", strconv.Itoa(count))
	u.RawQuery = query.Encode()

	resp, err := http.Get((u.String()))
	if err != nil {
		logger.Printf("Error sending request to endpoint %s: %v\n", endpoint, err)
		return
	}
	defer resp.Body.Close()

	logger.Printf("Sent count to endpoint %s, status code: %d\n", u.String(), resp.StatusCode)
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

	// Record the unique request
	mu.Lock()
	uniqueRequests.Store(id, true)
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))

	if endpoint != "" {
		// Count unique requests
		mu.Lock()
		count := 0
		uniqueRequests.Range(func(_, _ interface{}) bool {
			count++
			return true
		})
		mu.Unlock()

		go sendCountToEndpoint(endpoint, count, log.Default())
	}
}

func main() {
	logger := initLogger()
	go logAndNotifyUniqueRequests(logger)

	http.HandleFunc("/api/verve/accept", acceptHandler)

	port := ":8080"
	log.Printf("Starting server on %s...\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
