package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// configuration env vars
var (
	logDir          = getEnv("LOG_DIR", "/mnt/logs")
	serverName      = getEnv("KYBER_SERVER_NAME", "Kyber Server")
	rateLimit       = getEnvInt("RATE_LIMIT_SECONDS", 5)
	disableRate     = getEnvBool("DISABLE_RATE_LIMIT", false)
	pollIntervalMS  = getEnvInt("LOG_POLL_INTERVAL", 500)
	enableDetection = getEnvBool("ENABLE_DETECTION", true)
	enableAction    = getEnvBool("ENABLE_ACTION", true)
	enableError     = getEnvBool("ENABLE_ERROR", true)
	enableInfo      = getEnvBool("ENABLE_INFO", true)
	lastSent        = make(map[string]time.Time)
	chatFilterRegex = regexp.MustCompile(`\[ChatFilter\] (.*)`)
)

// discord webhook env vars
var (
	defaultWebhook   = os.Getenv("DISCORD_WEBHOOK_URL")
	detectionWebhook = os.Getenv("DISCORD_WEBHOOK_DETECTION_URL")
	actionWebhook    = os.Getenv("DISCORD_WEBHOOK_ACTION_URL")
	errorWebhook     = os.Getenv("DISCORD_WEBHOOK_ERROR_URL")
	infoWebhook      = os.Getenv("DISCORD_WEBHOOK_INFO_URL")
)

// http client
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func main() {
	if defaultWebhook == "" {
		fmt.Println("DISCORD_WEBHOOK_URL is required")
		os.Exit(1)
	}

	fmt.Println("ChatFilter Discord Sidecar Started")
	fmt.Println("Polling interval:", pollIntervalMS, "ms")

	for {
		latest, err := getLatestLogFile(logDir)
		if err != nil {
			fmt.Println("Error finding log file:", err)
			time.Sleep(time.Duration(pollIntervalMS) * time.Millisecond)
			continue
		}

		tailFile(latest)
	}
}

// searches the kyber log directory for .log files
func getLatestLogFile(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "kyber-server_*.log"))
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no log files found")
	}

	sort.Strings(files)
	return files[len(files)-1], nil
}

// opens most recent file and polls it for changes
func tailFile(path string) {
	fmt.Println("Tailing:", path)

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	file.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(time.Duration(pollIntervalMS) * time.Millisecond)

			// Detect rotation
			latest, _ := getLatestLogFile(logDir)
			if latest != path {
				fmt.Println("Log rotation detected")
				return
			}

			continue
		}

		processLine(line)
	}
}

// processes each "[ChatFilter]" line in the log file
func processLine(line string) {
	if !strings.Contains(line, "[ChatFilter]") {
		return
	}

	match := chatFilterRegex.FindStringSubmatch(line)
	if len(match) < 2 {
		return
	}

	content := match[1]

	cleanMessage, title, color, eventType := classifyEvent(content) //_ = eventType

	// checks if each message type is enabled, return if disabled
	if (eventType == "detection" && !enableDetection) ||
		(eventType == "action" && !enableAction) ||
		(eventType == "error" && !enableError) ||
		(eventType == "info" && !enableInfo) {
		return
	}

	// independent rate limiting per event type
	if !disableRate {
		last, exists := lastSent[eventType]
		if exists && time.Since(last) < time.Duration(rateLimit)*time.Second {
			return
		}
	}

	// sends embed to discord webhook
	webhook := getWebhookForEvent(eventType)
	err := sendToDiscord(webhook, title, cleanMessage, color)
	if err == nil {
		lastSent[eventType] = time.Now()
	}

}

func classifyEvent(content string) (clean, title string, color int, eventType string) {
	switch {
	case strings.HasPrefix(content, "Detection:"):
		return strings.TrimSpace(strings.TrimPrefix(content, "Detection:")),
			"ChatFilter Detection",
			10038562,
			"detection"

	case strings.HasPrefix(content, "Action:"):
		return strings.TrimSpace(strings.TrimPrefix(content, "Action:")),
			"ChatFilter Action",
			16753920,
			"action"

	case strings.HasPrefix(content, "Error:"):
		return strings.TrimSpace(strings.TrimPrefix(content, "Error:")),
			"ChatFilter Error",
			15158332,
			"error"

	default:
		return content,
			"ChatFilter Info",
			3447003,
			"info"
	}
}

// resolves webhook env vars
func getWebhookForEvent(eventType string) string {
	switch eventType {
	case "detection":
		if detectionWebhook != "" {
			return detectionWebhook
		}
	case "action":
		if actionWebhook != "" {
			return actionWebhook
		}
	case "error":
		if errorWebhook != "" {
			return errorWebhook
		}
	case "info":
		if infoWebhook != "" {
			return infoWebhook
		}
	}
	return defaultWebhook
}

func sendToDiscord(webhook, title, message string, color int) error {
	payload := map[string]interface{}{
		"allowed_mentions": map[string]interface{}{
			"parse": []string{},
		},
		"embeds": []map[string]interface{}{
			{
				"author": map[string]interface{}{
					"name": serverName,
				},
				"title":       title,
				"description": message,
				"color":       color,
				"timestamp":   time.Now().Format(time.RFC3339),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("Webhook error:", err)
		return err
	}
	defer resp.Body.Close()

	// handles discord rate limiting 429 response
	if resp.StatusCode == 429 {
		retry := resp.Header.Get("Retry-After")
		if retry != "" {
			if seconds, err := strconv.ParseFloat(retry, 64); err == nil {
				time.Sleep(time.Duration(seconds * float64(time.Second)))
			}
		}
		return fmt.Errorf("rate limited")
	}

	if resp.StatusCode >= 300 {
		fmt.Println("Discord returned:", resp.Status)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	val, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return val
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "true" || v == "1" {
		return true
	}
	if v == "false" || v == "0" {
		return false
	}
	return fallback
}
