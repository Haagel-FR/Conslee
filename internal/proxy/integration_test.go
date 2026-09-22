package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"conslee/internal/config"
)

// mockContainerRuntime is a test double for ContainerRuntime.
type mockContainerRuntime struct {
	mu         sync.Mutex
	containers map[string]ContainerState
	infos      []ContainerInfo
	startCalls []string
	stopCalls  []string
}

func newMockRuntime() *mockContainerRuntime {
	return &mockContainerRuntime{
		containers: make(map[string]ContainerState),
	}
}

func (m *mockContainerRuntime) Inspect(ctx context.Context, name string) (ContainerState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.containers[name]; ok {
		return state, nil
	}
	return ContainerState{Running: false}, nil
}

func (m *mockContainerRuntime) Start(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalls = append(m.startCalls, name)
	m.containers[name] = ContainerState{Running: true}
	return nil
}

func (m *mockContainerRuntime) Stop(ctx context.Context, name string, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalls = append(m.stopCalls, name)
	m.containers[name] = ContainerState{Running: false}
	return nil
}

func (m *mockContainerRuntime) List(ctx context.Context, all bool) ([]ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.infos, nil
}

// newTestConslee creates a Conslee instance with a mock runtime and configPath=""
// (disabling file save) for testing HTTP handlers.
func newTestConslee(t *testing.T) *Conslee {
	t.Helper()
	rt := newMockRuntime()
	cfg := &config.Config{
		Server: config.ServerConfig{ListenAddr: ":8800"},
		IdleReaper: config.IdleReaperConfig{
			RawInterval: "1m",
			Interval:    time.Minute,
		},
		Services: []config.ServiceConfig{},
	}
	reg := NewRegistry()
	return &Conslee{
		rt:         rt,
		reg:        reg,
		cfg:        cfg,
		configPath: "",
	}
}

// Tests

func TestIntegration_CreateListDeleteService(t *testing.T) {
	c := newTestConslee(t)

	// Create
	createReq := CreateServiceRequest{
		Name:           "webapp",
		Host:           "webapp.local",
		Containers:     []string{"webapp-container"},
		TargetURL:      "http://localhost:3000",
		Mode:           "on_demand",
		IdleTimeout:    "0s",
		StartupTimeout: "30s",
	}
	body, _ := json.Marshal(createReq)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.HandleCreateService(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List
	rec = httptest.NewRecorder()
	c.HandleListServices(rec, httptest.NewRequest(http.MethodGet, "/api/services", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}
	var services []ServiceStatusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &services); err != nil {
		t.Fatalf("unmarshal services: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Name != "webapp" {
		t.Errorf("expected name 'webapp', got %q", services[0].Name)
	}

	// Delete
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/services/webapp", nil)
	c.HandleDeleteService(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// List again - should be empty
	rec = httptest.NewRecorder()
	c.HandleListServices(rec, httptest.NewRequest(http.MethodGet, "/api/services", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &services); err != nil {
		t.Fatalf("unmarshal services: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected 0 services after delete, got %d", len(services))
	}
}

func TestIntegration_CreateServiceDuplicateName(t *testing.T) {
	c := newTestConslee(t)

	createReq := CreateServiceRequest{
		Name:           "svc1",
		Host:           "svc1.local",
		Containers:     []string{"c1"},
		TargetURL:      "http://localhost:3001",
		Mode:           "on_demand",
		StartupTimeout: "30s",
	}
	body, _ := json.Marshal(createReq)

	// First create succeeds
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.HandleCreateService(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", rec.Code)
	}

	// Second create with same name fails with 409
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.HandleCreateService(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name create: expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIntegration_CreateServiceDuplicateHost(t *testing.T) {
	c := newTestConslee(t)

	// Create first service
	createReq1 := CreateServiceRequest{
		Name:           "svc1",
		Host:           "shared.local",
		Containers:     []string{"c1"},
		TargetURL:      "http://localhost:3001",
		Mode:           "on_demand",
		StartupTimeout: "30s",
	}
	body1, _ := json.Marshal(createReq1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/services", bytes.NewReader(body1))
	req.Header.Set("Content-Type", "application/json")
	c.HandleCreateService(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d", rec.Code)
	}

	// Create second service with different name but same host
	createReq2 := CreateServiceRequest{
		Name:           "svc2",
		Host:           "shared.local",
		Containers:     []string{"c2"},
		TargetURL:      "http://localhost:3002",
		Mode:           "on_demand",
		StartupTimeout: "30s",
	}
	body2, _ := json.Marshal(createReq2)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/services", bytes.NewReader(body2))
	req.Header.Set("Content-Type", "application/json")
	c.HandleCreateService(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate host create: expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIntegration_StartStopService(t *testing.T) {
	c := newTestConslee(t)
	rt := c.rt.(*mockContainerRuntime)

	// Add service directly to registry with no TargetURL (avoids waitTCP/waitHTTP in ensureRunning)
	c.reg.Add("startstop.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:       "startstop",
			Host:       "startstop.local",
			Containers: []string{"startstop-container"},
			Mode:       "on_demand",
			TargetURL:  "",
		},
		LastActivity: time.Now(),
	})

	// Start
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/services/startstop/start", nil)
	c.HandleStartService(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("start: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	rt.mu.Lock()
	if len(rt.startCalls) != 1 || rt.startCalls[0] != "startstop-container" {
		t.Fatalf("expected startCalls=[startstop-container], got %v", rt.startCalls)
	}
	rt.mu.Unlock()

	// Stop
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/services/startstop/stop", nil)
	c.HandleStopService(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("stop: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	rt.mu.Lock()
	if len(rt.stopCalls) != 1 || rt.stopCalls[0] != "startstop-container" {
		t.Fatalf("expected stopCalls=[startstop-container], got %v", rt.stopCalls)
	}
	rt.mu.Unlock()
}

func TestIntegration_UpdateService(t *testing.T) {
	c := newTestConslee(t)

	// Create service first
	c.reg.Add("updatetest.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:       "updatetest",
			Host:       "updatetest.local",
			Containers: []string{"old-container"},
			Mode:       "on_demand",
			TargetURL:  "http://localhost:4000",
		},
		LastActivity: time.Now(),
	})

	// Update mode, idleTimeout, containers, and host
	newMode := "schedule_only"
	newIdle := "5m"
	newContainers := []string{"new-container"}
	newHost := "updated.local"
	emptyHeaders := ""

	updateReq := UpdateServiceRequest{
		Mode:        &newMode,
		IdleTimeout: &newIdle,
		Containers:  &newContainers,
		Host:        &newHost,
		CustomHeaders: &emptyHeaders,
	}
	body, _ := json.Marshal(updateReq)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/services/updatetest/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.HandleUpdateService(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("update: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify changes
	svc, ok := c.reg.GetByName("updatetest")
	if !ok {
		t.Fatal("service not found after update")
	}
	if svc.Config.Mode != "schedule_only" {
		t.Errorf("expected mode 'schedule_only', got %q", svc.Config.Mode)
	}
	if svc.Config.IdleTimeout != 5*time.Minute {
		t.Errorf("expected idleTimeout 5m, got %v", svc.Config.IdleTimeout)
	}
	if len(svc.Config.Containers) != 1 || svc.Config.Containers[0] != "new-container" {
		t.Errorf("expected containers [new-container], got %v", svc.Config.Containers)
	}
	if svc.Config.Host != "updated.local" {
		t.Errorf("expected host 'updated.local', got %q", svc.Config.Host)
	}

	// Verify old host is removed from lookup
	if _, ok := c.reg.GetByHost("updatetest.local"); ok {
		t.Error("old host should not be in registry after update")
	}
	// Verify new host is in lookup
	if _, ok := c.reg.GetByHost("updated.local"); !ok {
		t.Error("new host should be in registry after update")
	}
}

func TestIntegration_SystemEndpoints(t *testing.T) {
	c := newTestConslee(t)

	// Get system status
	rec := httptest.NewRecorder()
	c.HandleGetSystem(rec, httptest.NewRequest(http.MethodGet, "/api/system", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get system: expected 200, got %d", rec.Code)
	}
	var sys SystemStatusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &sys); err != nil {
		t.Fatalf("unmarshal system: %v", err)
	}
	if sys.ListenAddr != ":8800" {
		t.Errorf("expected listenAddr :8800, got %q", sys.ListenAddr)
	}
	if sys.IdleReaperInterval != "1m0s" {
		t.Errorf("expected idleReaperInterval 1m0s, got %q", sys.IdleReaperInterval)
	}

	// Update system
	newAddr := ":9900"
	newInterval := "2m"
	updateReq := UpdateSystemRequest{
		ListenAddr:         &newAddr,
		IdleReaperInterval: &newInterval,
	}
	body, _ := json.Marshal(updateReq)
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/system", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.HandleUpdateSystem(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("update system: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify update
	rec = httptest.NewRecorder()
	c.HandleGetSystem(rec, httptest.NewRequest(http.MethodGet, "/api/system", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &sys); err != nil {
		t.Fatalf("unmarshal system: %v", err)
	}
	if sys.ListenAddr != ":9900" {
		t.Errorf("expected updated listenAddr :9900, got %q", sys.ListenAddr)
	}
	if sys.IdleReaperInterval != "2m0s" {
		t.Errorf("expected updated idleReaperInterval 2m0s, got %q", sys.IdleReaperInterval)
	}
}

func TestIntegration_DockerListEndpoint(t *testing.T) {
	c := newTestConslee(t)
	rt := c.rt.(*mockContainerRuntime)

	// Seed mock data
	rt.mu.Lock()
	rt.infos = []ContainerInfo{
		{
			ID:     "abc123",
			Name:   "test-container",
			Image:  "nginx:latest",
			State:  "running",
			Status: "Up 2 hours",
			Ports: []Port{
				{IP: "0.0.0.0", Private: 80, Public: 8080, Type: "tcp"},
			},
			Stack: "mystack",
		},
		{
			ID:     "def456",
			Name:   "db-container",
			Image:  "postgres:15",
			State:  "exited",
			Status: "Exited (0) 1 hour ago",
			Ports:  []Port{},
			Stack:  "mystack",
		},
	}
	rt.mu.Unlock()

	rec := httptest.NewRecorder()
	c.HandleListContainers(rec, httptest.NewRequest(http.MethodGet, "/api/docker/containers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("docker list: expected 200, got %d", rec.Code)
	}

	var containers []DockerContainerDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &containers); err != nil {
		t.Fatalf("unmarshal containers: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}
	if containers[0].Name != "test-container" {
		t.Errorf("expected name 'test-container', got %q", containers[0].Name)
	}
	if containers[0].Stack != "mystack" {
		t.Errorf("expected stack 'mystack', got %q", containers[0].Stack)
	}
	if len(containers[0].Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(containers[0].Ports))
	}
	if containers[0].Ports[0].Private != 80 {
		t.Errorf("expected private port 80, got %d", containers[0].Ports[0].Private)
	}
}

func TestIntegration_NotFoundCases(t *testing.T) {
	c := newTestConslee(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"delete nonexistent", http.MethodDelete, "/api/services/nonexistent"},
		{"start nonexistent", http.MethodPost, "/api/services/nonexistent/start"},
		{"stop nonexistent", http.MethodPost, "/api/services/nonexistent/stop"},
		{"update nonexistent", http.MethodPost, "/api/services/nonexistent/settings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			var req *http.Request
			if tt.method == http.MethodPost && tt.path[len(tt.path)-len("/settings"):] == "/settings" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte(`{}`)))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			suffix := ""
			if len(tt.path) > len("/api/services/") {
				rest := tt.path[len("/api/services/"):]
				if len(rest) >= len("/start") && rest[len(rest)-len("/start"):] == "/start" {
					suffix = "/start"
				} else if len(rest) >= len("/stop") && rest[len(rest)-len("/stop"):] == "/stop" {
					suffix = "/stop"
				} else if len(rest) >= len("/settings") && rest[len(rest)-len("/settings"):] == "/settings" {
					suffix = "/settings"
				}
			}

			switch {
			case tt.method == http.MethodDelete:
				c.HandleDeleteService(rec, req)
			case tt.method == http.MethodPost && suffix == "/start":
				c.HandleStartService(rec, req)
			case tt.method == http.MethodPost && suffix == "/stop":
				c.HandleStopService(rec, req)
			case tt.method == http.MethodPost && suffix == "/settings":
				c.HandleUpdateService(rec, req)
			}

			if rec.Code != http.StatusNotFound {
				t.Errorf("%s: expected 404, got %d: %s", tt.name, rec.Code, rec.Body.String())
			}
		})
	}
}
