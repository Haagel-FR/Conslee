package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"conslee/internal/config"
)

func TestStartupPage_RedirectsToStartupPath(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	targetURL, _ := url.Parse("http://localhost:3000")
	c.reg.Add("webapp.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:           "webapp",
			Host:           "webapp.local",
			Containers:     []string{"webapp-container"},
			TargetURL:      "http://localhost:3000",
			Mode:           "on_demand",
			StartupTimeout: 30 * time.Second,
		},
		Target:       targetURL,
		LastActivity: time.Now(),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	req.Host = "webapp.local"

	c.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, StartupPath) {
		t.Errorf("expected redirect to %s, got %s", StartupPath, location)
	}
	if !strings.Contains(location, "path=%2F") {
		t.Errorf("expected path query param in Location, got %s", location)
	}
	if !strings.Contains(location, "service=webapp") {
		t.Errorf("expected service query param in Location, got %s", location)
	}
	if !strings.Contains(location, "target=http") {
		t.Errorf("expected target query param in Location, got %s", location)
	}
}

func TestStartupPage_Handler(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	targetURL, _ := url.Parse("http://localhost:3000")
	c.reg.Add("webapp.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:           "webapp",
			Host:           "webapp.local",
			Containers:     []string{"webapp-container"},
			TargetURL:      "http://localhost:3000",
			Mode:           "on_demand",
			StartupTimeout: 30 * time.Second,
		},
		Target:       targetURL,
		LastActivity: time.Now(),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, StartupPath+"?path=%2Fdashboard&service=webapp", nil)

	c.HandleStartupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "loading-container") {
		t.Errorf("expected loading-container in HTML, got: %s", body)
	}
	if !strings.Contains(body, "window.location.href") {
		t.Errorf("expected window.location.href in HTML, got: %s", body)
	}
	if !strings.Contains(body, "Starting webapp") {
		t.Errorf("expected 'Starting webapp' in HTML, got: %s", body)
	}
	if !strings.Contains(body, "/api/probes") {
		t.Errorf("expected /api/probes POST in HTML, got: %s", body)
	}
	if !strings.Contains(body, "requireSignature") {
		t.Errorf("expected 'requireSignature' in HTML polling logic, got: %s", body)
	}

	// 3-step progress assertions
	if !strings.Contains(body, "loading-steps") {
		t.Errorf("expected 'loading-steps' class in HTML, got: %s", body)
	}
	if !strings.Contains(body, "loading-step") {
		t.Errorf("expected 'loading-step' class in HTML, got: %s", body)
	}
	if !strings.Contains(body, "loading-step-active") {
		t.Errorf("expected 'loading-step-active' class in HTML, got: %s", body)
	}
	if !strings.Contains(body, "loading-step-completed") {
		t.Errorf("expected 'loading-step-completed' in HTML, got: %s", body)
	}
	if !strings.Contains(body, "Starting service...") {
		t.Errorf("expected 'Starting service...' step text in HTML, got: %s", body)
	}
	if !strings.Contains(body, "Waiting service...") {
		t.Errorf("expected 'Waiting service...' step text in HTML, got: %s", body)
	}
	if !strings.Contains(body, "Redirecting...") {
		t.Errorf("expected 'Redirecting...' step text in HTML, got: %s", body)
	}
	if !strings.Contains(body, "step-starting") {
		t.Errorf("expected 'step-starting' id in HTML, got: %s", body)
	}
	if !strings.Contains(body, "step-waiting") {
		t.Errorf("expected 'step-waiting' id in HTML, got: %s", body)
	}
	if !strings.Contains(body, "step-redirecting") {
		t.Errorf("expected 'step-redirecting' id in HTML, got: %s", body)
	}
}

func TestStartupPage_RedirectWithCustomHeaders(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	customHeaders := `[{"Authorization":"Bearer my-token"}]`
	targetURL, _ := url.Parse("http://localhost:3000")
	c.reg.Add("webapp.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:           "webapp",
			Host:           "webapp.local",
			Containers:     []string{"webapp-container"},
			TargetURL:      "http://localhost:3000",
			CustomHeaders:  customHeaders,
			Mode:           "on_demand",
			StartupTimeout: 30 * time.Second,
		},
		Target:       targetURL,
		LastActivity: time.Now(),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	req.Host = "webapp.local"

	c.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, StartupPath) {
		t.Errorf("expected redirect to %s, got %s", StartupPath, location)
	}
	if !strings.Contains(location, "customHeaders=") {
		t.Errorf("expected customHeaders query param in Location, got %s", location)
	}
	if !strings.Contains(location, "Bearer") {
		t.Errorf("expected customHeaders value in Location, got %s", location)
	}
}

func TestStartupPage_HandlerWithCustomHeaders(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	customHeaders := `[{"Authorization":"Bearer my-token"}]`
	q := url.Values{}
	q.Set("path", "/dashboard")
	q.Set("service", "webapp")
	q.Set("customHeaders", customHeaders)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, StartupPath+"?"+q.Encode(), nil)

	c.HandleStartupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	// JS reads customHeaders from URL params
	if !strings.Contains(body, "params.get('customHeaders')") {
		t.Errorf("expected JS to read customHeaders from URL params, got: %s", body)
	}
	// JS includes customHeaders in the probe body
	if !strings.Contains(body, "body.customHeaders") {
		t.Errorf("expected body.customHeaders in JS, got: %s", body)
	}
	// The conditional check pattern
	if !strings.Contains(body, "if (customHeaders)") {
		t.Errorf("expected 'if (customHeaders)' check in JS, got: %s", body)
	}
}

func TestStartupPage_HandlerWithPath(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, StartupPath+"?path=%2Fsome%2Fpage&service=nonexistent", nil)

	c.HandleStartupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "loading-container") {
		t.Errorf("expected loading-container in HTML, got: %s", body)
	}
	if !strings.Contains(body, "Starting Service") {
		t.Errorf("expected 'Starting Service' in HTML, got: %s", body)
	}
}

func TestStartupPage_HandlerNoPath(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, StartupPath, nil)

	c.HandleStartupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "loading-container") {
		t.Errorf("expected loading-container in HTML, got: %s", body)
	}
}

func TestStartupPage_WrongPath(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wrong-path", nil)

	c.HandleStartupPage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestLoadingPage_NotShownForAPIRequest(t *testing.T) {
	c := newTestConslee(t)
	_ = c.rt.(*mockContainerRuntime)

	targetURL, _ := url.Parse("http://localhost:3000")
	c.reg.Add("webapp.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:           "webapp",
			Host:           "webapp.local",
			Containers:     []string{"webapp-container"},
			TargetURL:      "http://localhost:3000",
			Mode:           "on_demand",
			StartupTimeout: 30 * time.Second,
		},
		Target:       targetURL,
		LastActivity: time.Now(),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/json")
	req.Host = "webapp.local"

	c.ServeHTTP(rec, req)

	if rec.Code == http.StatusFound {
		t.Errorf("API request should not redirect to startup page, got redirect to: %s", rec.Header().Get("Location"))
	}
}

func TestLoadingPage_NotShownWhenAlreadyRunning(t *testing.T) {
	c := newTestConslee(t)
	rt := c.rt.(*mockContainerRuntime)

	targetURL, _ := url.Parse("http://localhost:3000")
	c.reg.Add("webapp.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:           "webapp",
			Host:           "webapp.local",
			Containers:     []string{"webapp-container"},
			TargetURL:      "http://localhost:3000",
			Mode:           "on_demand",
			StartupTimeout: 30 * time.Second,
		},
		Target:       targetURL,
		LastActivity: time.Now(),
	})

	rt.mu.Lock()
	rt.containers["webapp-container"] = ContainerState{Running: true}
	rt.mu.Unlock()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Host = "webapp.local"

	c.ServeHTTP(rec, req)

	if rec.Code == http.StatusFound {
		t.Errorf("should not redirect to startup page when already running, got: %s", rec.Header().Get("Location"))
	}
}

func TestIsBrowserHTMLRequest(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{"browser", "text/html,application/xhtml+xml", true},
		{"browser with q", "text/html;q=0.9,application/json;q=0.8", true},
		{"api json", "application/json", false},
		{"api any", "*/*", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Accept", tt.accept)
			if got := isBrowserHTMLRequest(r); got != tt.want {
				t.Errorf("isBrowserHTMLRequest(%q) = %v, want %v", tt.accept, got, tt.want)
			}
		})
	}
}

func TestIsServiceReady(t *testing.T) {
	c := newTestConslee(t)
	rt := c.rt.(*mockContainerRuntime)

	targetURL, _ := url.Parse("http://localhost:3000")
	c.reg.Add("ready.local", &ServiceState{
		Config: config.ServiceConfig{
			Name:       "ready",
			Host:       "ready.local",
			Containers: []string{"container-a", "container-b"},
			TargetURL:  "http://localhost:3000",
			Mode:       "on_demand",
		},
		Target:       targetURL,
		LastActivity: time.Now(),
	})

	svc, ok := c.reg.GetByHost("ready.local")
	if !ok {
		t.Fatal("service not found")
	}

	if c.isServiceReady(svc) {
		t.Error("expected not ready when no containers running")
	}

	rt.mu.Lock()
	rt.containers["container-a"] = ContainerState{Running: true}
	rt.mu.Unlock()

	if !c.isServiceReady(svc) {
		t.Error("expected ready when at least one container running")
	}
}
