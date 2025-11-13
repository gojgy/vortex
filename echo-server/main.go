package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type ResponseData struct {
	InstanceID     string      `json:"instance_id"`
	RequestPath    string      `json:"request_path"`
	RequestCount   int64       `json:"request_count"`
	AdditionalInfo interface{} `json:"additional_info,omitempty"`
}

var requestCounter int64
var instanceID string

func main() {
	instanceID = os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "unknown"
	}

	http.HandleFunc("/", handler)

	log.Printf("Starting echo-server with ID: %s on port 80", instanceID)
	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	requestCounter++
	w.Header().Set("Content-Type", "application/json")

	if strings.HasPrefix(r.URL.Path, "/hold/") {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 3 {
			seconds, err := strconv.Atoi(parts[2])
			if err == nil {
				time.Sleep(time.Duration(seconds) * time.Second)
			}
		}
	}

	response := ResponseData{
		InstanceID:   instanceID,
		RequestPath:  r.URL.Path,
		RequestCount: requestCounter,
	}

	json.NewEncoder(w).Encode(response)
}
