package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"conslee/internal/config"
)

// setupE2EServer creates an httptest.Server with a mux mirroring main.go routing.
// Returns the server, the mock runtime, and the base URL.
func setupE2EServer(t *testing.T) (*httptest.Server, *mockContainerRuntime, string) {
	t.Helper()

	rt := newMockRuntime()
	reg := NewRegistry()

	c := &Conslee{
		rt:  rt,
		reg: reg,
		cfg: &config.Config{
			Server: config.ServerConfig{
				ListenAddr: ":8800",
			},
			IdleReaper: config.IdleReaperConfig{
				RawInterval: "1m",
				Interval:    time.Minute,
			},
			Services: []config.ServiceConfig{},
		},
		configPath: "",
	}

	mux := http.NewServeMux()

	// /api/services
	mux.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			c.HandleListServices(w, r)
		case http.MethodPost:
			c.HandleCreateService(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// /api/services/... – start/stop/settings/delete
	mux.HandleFunc("/api/services/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasSuffix(path, "/start") && r.Method == http.MethodPost {
			c.HandleStartService(w, r)
			return
		}
		if strings.HasSuffix(path, "/stop") && r.Method == http.MethodPost {
			c.HandleStopService(w, r)
			return
		}
		if strings.HasSuffix(path, "/settings") && r.Method == http.MethodPost {
			c.HandleUpdateService(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			c.HandleDeleteService(w, r)
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	})

	// /api/docker/containers
	mux.HandleFunc("/api/docker/containers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c.HandleListContainers(w, r)
	})

	// GET /api/system, POST /api/system
	mux.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			c.HandleGetSystem(w, r)
		case http.MethodPost:
			c.HandleUpdateSystem(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// GET /api/system/check-port
	mux.HandleFunc("/api/system/check-port", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c.HandleCheckPort(w, r)
	})

	// POST /api/probes
	mux.HandleFunc("/api/probes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c.HandleProbe(w, r)
	})

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Reverse proxy
	mux.Handle("/", c)

	server := httptest.NewServer(mux)
	return server, rt, server.URL
}

// doRequest is a helper to make HTTP requests and read the response body.
func doRequest(t *testing.T, client *http.Client, method, url string, body []byte) (int, []byte) {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	return resp.StatusCode, respBody
}

// TestE2E_FullServiceLifecycle tests create → list → update → delete flow.
func TestE2E_FullServiceLifecycle(t *testing.T) {
	server, _, baseURL := setupE2EServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Create a service
	createBody := map[string]interface{}{
		"name":           "lifecycle-test",
		"host":           "lifecycle.example.com",
		"targetUrl":      "http://localhost:9999",
		"containers":     []string{"lifecycle-container"},
		"startupTimeout": "30s",
	}
	bodyBytes, _ := json.Marshal(createBody)

	status, respBody := doRequest(t, client, http.MethodPost, baseURL+"/api/services", bodyBytes)
	if status != http.StatusCreated {
		t.Fatalf("create service: expected 201, got %d: %s", status, string(respBody))
	}

	// 2. List services and verify the new one is present
	status, respBody = doRequest(t, client, http.MethodGet, baseURL+"/api/services", nil)
	if status != http.StatusOK {
		t.Fatalf("list services: expected 200, got %d: %s", status, string(respBody))
	}

	var services []ServiceStatusDTO
	if err := json.Unmarshal(respBody, &services); err != nil {
		t.Fatalf("failed to unmarshal service list: %v", err)
	}

	found := false
	for _, svc := range services {
		if svc.Name == "lifecycle-test" {
			found = true
			if svc.Host != "lifecycle.example.com" {
				t.Errorf("expected host lifecycle.example.com, got %s", svc.Host)
			}
			break
		}
	}
	if !found {
		t.Fatal("service 'lifecycle-test' not found in list")
	}

	// 3. Update service settings
	updateBody := map[string]interface{}{
		"mode":        "schedule_only",
		"customHeaders": "",
	}
	bodyBytes, _ = json.Marshal(updateBody)

	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/services/lifecycle-test/settings", bodyBytes)
	if status != http.StatusNoContent {
		t.Fatalf("update service: expected 204, got %d: %s", status, string(respBody))
	}

	// 4. Verify update by listing again
	status, respBody = doRequest(t, client, http.MethodGet, baseURL+"/api/services", nil)
	if status != http.StatusOK {
		t.Fatalf("list services after update: expected 200, got %d: %s", status, string(respBody))
	}

	if err := json.Unmarshal(respBody, &services); err != nil {
		t.Fatalf("failed to unmarshal service list after update: %v", err)
	}

	for _, svc := range services {
		if svc.Name == "lifecycle-test" {
			if svc.Mode != "schedule_only" {
				t.Errorf("expected mode schedule_only, got %s", svc.Mode)
			}
			break
		}
	}

	// 5. Delete the service
	status, respBody = doRequest(t, client, http.MethodDelete, baseURL+"/api/services/lifecycle-test", nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete service: expected 204, got %d: %s", status, string(respBody))
	}

	// 6. Verify deletion
	status, respBody = doRequest(t, client, http.MethodGet, baseURL+"/api/services", nil)
	if status != http.StatusOK {
		t.Fatalf("list services after delete: expected 200, got %d: %s", status, string(respBody))
	}

	if err := json.Unmarshal(respBody, &services); err != nil {
		t.Fatalf("failed to unmarshal service list after delete: %v", err)
	}

	for _, svc := range services {
		if svc.Name == "lifecycle-test" {
			t.Fatal("service 'lifecycle-test' still exists after deletion")
		}
	}
}

// TestE2E_InvalidRequests tests that invalid inputs return proper error codes.
func TestE2E_InvalidRequests(t *testing.T) {
	server, _, baseURL := setupE2EServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	tests := []struct {
		name       string
		method     string
		path       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name:   "missing name",
			method: http.MethodPost,
			path:   "/api/services",
			body: map[string]interface{}{
				"host":      "test.example.com",
				"targetUrl": "http://localhost:9999",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "missing host for on_demand mode",
			method: http.MethodPost,
			path:   "/api/services",
			body: map[string]interface{}{
				"name":      "no-host-test",
				"targetUrl": "http://localhost:9999",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "missing targetUrl for on_demand mode",
			method: http.MethodPost,
			path:   "/api/services",
			body: map[string]interface{}{
				"name": "no-target-test",
				"host": "test.example.com",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid JSON",
			method: http.MethodPost,
			path:   "/api/services",
			body:   nil, // will send garbage
			wantStatus: http.StatusBadRequest,
		},

		{
			name:   "update nonexistent service",
			method: http.MethodPost,
			path:   "/api/services/does-not-exist/settings",
			body: map[string]interface{}{
				"mode":        "schedule_only",
		"customHeaders": "",
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "delete nonexistent service",
			method: http.MethodDelete,
			path:   "/api/services/does-not-exist",
			body:   nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "start nonexistent service",
			method: http.MethodPost,
			path:   "/api/services/does-not-exist/start",
			body:   nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "stop nonexistent service",
			method: http.MethodPost,
			path:   "/api/services/does-not-exist/stop",
			body:   nil,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("not valid json{{{")
			}

			var reqBody io.Reader
			if body != nil {
				reqBody = bytes.NewReader(body)
			}

			req, err := http.NewRequest(tt.method, baseURL+tt.path, reqBody)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				respBody, _ := io.ReadAll(resp.Body)
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, resp.StatusCode, string(respBody))
			}
		})
	}

	// Test duplicate service creation
	createBody := map[string]interface{}{
		"name":           "duplicate-test",
		"host":           "duplicate.example.com",
		"targetUrl":      "http://localhost:9999",
		"containers":     []string{"dup-container"},
		"startupTimeout": "30s",
	}
	bodyBytes, _ := json.Marshal(createBody)

	status, _ := doRequest(t, client, http.MethodPost, baseURL+"/api/services", bodyBytes)
	if status != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", status)
	}

	status, respBody := doRequest(t, client, http.MethodPost, baseURL+"/api/services", bodyBytes)
	if status != http.StatusConflict {
		t.Fatalf("duplicate create: expected 409, got %d: %s", status, string(respBody))
	}

	// Test duplicate host
	dupHostBody := map[string]interface{}{
		"name":           "dup-host-test",
		"host":           "duplicate.example.com",
		"targetUrl":      "http://localhost:9998",
		"containers":     []string{"dup-host-container"},
		"startupTimeout": "30s",
	}
	bodyBytes, _ = json.Marshal(dupHostBody)

	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/services", bodyBytes)
	if status != http.StatusConflict {
		t.Fatalf("duplicate host: expected 409, got %d: %s", status, string(respBody))
	}
}

// TestE2E_SystemPortChange tests system config and port checking.
func TestE2E_SystemPortChange(t *testing.T) {
	server, _, baseURL := setupE2EServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Get system config
	status, respBody := doRequest(t, client, http.MethodGet, baseURL+"/api/system", nil)
	if status != http.StatusOK {
		t.Fatalf("get system: expected 200, got %d: %s", status, string(respBody))
	}

	var sys SystemStatusDTO
	if err := json.Unmarshal(respBody, &sys); err != nil {
		t.Fatalf("failed to unmarshal system status: %v", err)
	}

	if sys.ListenAddr != ":8800" {
		t.Errorf("expected listenAddr :8800, got %s", sys.ListenAddr)
	}
	if sys.IdleReaperInterval != "1m0s" {
		t.Errorf("expected idleReaperInterval 1m0s, got %s", sys.IdleReaperInterval)
	}

	// 2. Check port availability (current port — should report available since it matches config)
	status, respBody = doRequest(t, client, http.MethodGet, baseURL+"/api/system/check-port?listenAddr="+":8800", nil)
	if status != http.StatusOK {
		t.Fatalf("check-port: expected 200, got %d: %s", status, string(respBody))
	}

	// 3. Update system config — change port
	updateBody := map[string]interface{}{
		"listenAddr": ":9900",
	}
	bodyBytes, _ := json.Marshal(updateBody)

	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/system", bodyBytes)
	if status != http.StatusNoContent {
		t.Fatalf("update system: expected 204, got %d: %s", status, string(respBody))
	}

	// 4. Verify the change
	status, respBody = doRequest(t, client, http.MethodGet, baseURL+"/api/system", nil)
	if status != http.StatusOK {
		t.Fatalf("get system after update: expected 200, got %d: %s", status, string(respBody))
	}

	if err := json.Unmarshal(respBody, &sys); err != nil {
		t.Fatalf("failed to unmarshal system status after update: %v", err)
	}

	if sys.ListenAddr != ":9900" {
		t.Errorf("expected listenAddr :9900, got %s", sys.ListenAddr)
	}

	// 5. Test invalid listenAddr format
	updateBody = map[string]interface{}{
		"listenAddr": "not-a-port",
	}
	bodyBytes, _ = json.Marshal(updateBody)

	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/system", bodyBytes)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid listenAddr: expected 400, got %d: %s", status, string(respBody))
	}

	// 6. Test invalid idleReaperInterval
	updateBody = map[string]interface{}{
		"idleReaperInterval": "not-a-duration",
	}
	bodyBytes, _ = json.Marshal(updateBody)

	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/system", bodyBytes)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid idleReaperInterval: expected 400, got %d: %s", status, string(respBody))
	}
}

// TestE2E_ProbeEndpoint tests the probe endpoint with various scenarios.
func TestE2E_ProbeEndpoint(t *testing.T) {
	// Create a target server to probe
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("target ok"))
	}))
	defer targetServer.Close()

	server, _, baseURL := setupE2EServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Probe the healthy target server
	probeBody := ProbeRequest{
		URL:       targetServer.URL,
		AllowWake: true,
	}
	bodyBytes, _ := json.Marshal(probeBody)

	status, respBody := doRequest(t, client, http.MethodPost, baseURL+"/api/probes", bodyBytes)
	if status != http.StatusOK {
		t.Fatalf("probe: expected 200, got %d: %s", status, string(respBody))
	}

	var probeResp ProbeResponse
	if err := json.Unmarshal(respBody, &probeResp); err != nil {
		t.Fatalf("failed to unmarshal probe response: %v", err)
	}

	if probeResp.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", probeResp.Status)
	}
	if probeResp.StatusCode < 200 || probeResp.StatusCode >= 300 {
		t.Errorf("expected 2xx status code, got %d", probeResp.StatusCode)
	}

	// 2. Probe with empty URL (should fail validation)
	probeBody = ProbeRequest{
		URL: "",
	}
	bodyBytes, _ = json.Marshal(probeBody)

	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/probes", bodyBytes)
	if status != http.StatusBadRequest {
		t.Fatalf("probe empty url: expected 400, got %d: %s", status, string(respBody))
	}

	// 3. Probe with invalid JSON
	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/probes", []byte("bad json"))
	if status != http.StatusBadRequest {
		t.Fatalf("probe invalid json: expected 400, got %d: %s", status, string(respBody))
	}

	// 4. Probe with invalid JSON body (missing URL field)
	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/probes", []byte(`{"allowWake": true}`))
	if status != http.StatusBadRequest {
		t.Fatalf("probe missing url: expected 400, got %d: %s", status, string(respBody))
	}

	// 5. Probe with unreachable URL
	probeBody = ProbeRequest{
		URL:       "http://127.0.0.1:1", // port 1 should be unreachable
		AllowWake: false,
	}
	bodyBytes, _ = json.Marshal(probeBody)

	status, respBody = doRequest(t, client, http.MethodPost, baseURL+"/api/probes", bodyBytes)
	if status != http.StatusOK {
		t.Fatalf("probe unreachable: expected 200, got %d: %s", status, string(respBody))
	}

	if err := json.Unmarshal(respBody, &probeResp); err != nil {
		t.Fatalf("failed to unmarshal probe response: %v", err)
	}

	if probeResp.Status != "unhealthy" {
		t.Errorf("expected status unhealthy, got %s", probeResp.Status)
	}
}

// TestE2E_ConcurrentCreates tests that concurrent service creations work correctly.
func TestE2E_ConcurrentCreates(t *testing.T) {
	server, _, baseURL := setupE2EServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	const numGoroutines = 20

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			name := fmt.Sprintf("concurrent-%d", idx)
			host := fmt.Sprintf("concurrent-%d.example.com", idx)

			createBody := map[string]interface{}{
				"name":           name,
				"host":           host,
				"targetUrl":      "http://localhost:9999",
				"containers":     []string{fmt.Sprintf("container-%d", idx)},
				"startupTimeout": "30s",
			}
			bodyBytes, _ := json.Marshal(createBody)

			req, err := http.NewRequest(http.MethodPost, baseURL+"/api/services", bytes.NewReader(bodyBytes))
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: create request: %w", idx, err)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: request failed: %w", idx, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				respBody, _ := io.ReadAll(resp.Body)
				errCh <- fmt.Errorf("goroutine %d: expected 201, got %d: %s", idx, resp.StatusCode, string(respBody))
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}

	// Verify all services were created
	status, respBody := doRequest(t, client, http.MethodGet, baseURL+"/api/services", nil)
	if status != http.StatusOK {
		t.Fatalf("list services: expected 200, got %d: %s", status, string(respBody))
	}

	var services []ServiceStatusDTO
	if err := json.Unmarshal(respBody, &services); err != nil {
		t.Fatalf("failed to unmarshal service list: %v", err)
	}

	if len(services) != numGoroutines {
		t.Errorf("expected %d services, got %d", numGoroutines, len(services))
	}

	// Verify no duplicates and all names are present
	names := make(map[string]bool, len(services))
	for _, svc := range services {
		if names[svc.Name] {
			t.Errorf("duplicate service name: %s", svc.Name)
		}
		names[svc.Name] = true
	}

	for i := 0; i < numGoroutines; i++ {
		expected := fmt.Sprintf("concurrent-%d", i)
		if !names[expected] {
			t.Errorf("service %s not found", expected)
		}
	}
}
