package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

var baseURL string

func TestMain(m *testing.M) {
	baseURL = os.Getenv("API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}

	// Wait for service to be ready
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}
		time.Sleep(time.Second)
	}

	os.Exit(m.Run())
}

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %q", body["status"])
	}
}

func TestTelemetryFlow(t *testing.T) {
	deviceID := fmt.Sprintf("test-device-%d", time.Now().UnixNano())

	// POST telemetry
	telemetry := map[string]interface{}{
		"device_id": deviceID,
		"type":      "temperature",
		"value":     25.5,
		"unit":      "celsius",
	}

	body, _ := json.Marshal(telemetry)
	resp, err := http.Post(baseURL+"/telemetry", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to POST telemetry: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("POST /telemetry: expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)

	if created["id"] == nil || created["id"] == "" {
		t.Error("expected ID in response")
	}

	// GET telemetry
	resp, err = http.Get(fmt.Sprintf("%s/telemetry?device_id=%s", baseURL, deviceID))
	if err != nil {
		t.Fatalf("failed to GET telemetry: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /telemetry: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var telemetryResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&telemetryResp)

	count := int(telemetryResp["count"].(float64))
	if count < 1 {
		t.Errorf("expected at least 1 telemetry entry, got %d", count)
	}
}

func TestCommandFlow(t *testing.T) {
	deviceID := fmt.Sprintf("test-device-%d", time.Now().UnixNano())

	// POST command
	command := map[string]interface{}{
		"device_id": deviceID,
		"type":      "reboot",
		"payload":   map[string]string{"reason": "test"},
	}

	body, _ := json.Marshal(command)
	resp, err := http.Post(baseURL+"/commands", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to POST command: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("POST /commands: expected %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)

	commandID := created["id"].(string)
	if commandID == "" {
		t.Fatal("expected command ID")
	}

	// GET commands
	resp, err = http.Get(fmt.Sprintf("%s/commands?device_id=%s", baseURL, deviceID))
	if err != nil {
		t.Fatalf("failed to GET commands: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /commands: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// DELETE (acknowledge) command
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/commands/%s?acknowledge=true", baseURL, commandID), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to DELETE command: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("DELETE /commands: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var ackResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&ackResp)

	if ackResp["status"] != "acknowledged" {
		t.Errorf("expected status 'acknowledged', got %q", ackResp["status"])
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		method   string
		body     string
		want     int
	}{
		{
			name:     "telemetry missing device_id",
			endpoint: "/telemetry",
			method:   http.MethodPost,
			body:     `{"type": "temperature"}`,
			want:     http.StatusBadRequest,
		},
		{
			name:     "telemetry missing type",
			endpoint: "/telemetry",
			method:   http.MethodPost,
			body:     `{"device_id": "test"}`,
			want:     http.StatusBadRequest,
		},
		{
			name:     "command missing device_id",
			endpoint: "/commands",
			method:   http.MethodPost,
			body:     `{"type": "reboot"}`,
			want:     http.StatusBadRequest,
		},
		{
			name:     "command missing type",
			endpoint: "/commands",
			method:   http.MethodPost,
			body:     `{"device_id": "test"}`,
			want:     http.StatusBadRequest,
		},
		{
			name:     "get commands missing device_id",
			endpoint: "/commands",
			method:   http.MethodGet,
			body:     "",
			want:     http.StatusBadRequest,
		},
		{
			name:     "get telemetry missing device_id",
			endpoint: "/telemetry",
			method:   http.MethodGet,
			body:     "",
			want:     http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			var err error

			if tt.method == http.MethodGet {
				resp, err = http.Get(baseURL + tt.endpoint)
			} else {
				resp, err = http.Post(baseURL+tt.endpoint, "application/json", bytes.NewBufferString(tt.body))
			}

			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.want {
				t.Errorf("expected status %d, got %d", tt.want, resp.StatusCode)
			}
		})
	}
}
