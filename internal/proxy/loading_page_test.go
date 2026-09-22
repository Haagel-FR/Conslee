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

func TestLoadingPage_ReturnsHTMLForBrowserRequest(t *testing.T) {
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Starting webapp") {
		t.Errorf("expected loading page with 'Starting webapp', got: %s", body)
	}
	if !strings.Contains(body, "window.location.href") {
		t.Errorf("expected window.location.href in loading page, got: %s", body)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type, got %s", rec.Header().Get("Content-Type"))
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

	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "loading-spinner") {
		t.Errorf("API request should not get loading page, got HTML: %s", rec.Body.String())
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

	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "loading-spinner") {
		t.Errorf("should not show loading page when already running, got: %s", rec.Body.String())
	}
}

func TestLoadingPage_ContainsSpinner(t *testing.T) {
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
	req.Header.Set("Accept", "text/html")
	req.Host = "webapp.local"

	c.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "loading-spinner") {
		t.Errorf("expected spinner in loading page, got: %s", body)
	}
	if !strings.Contains(body, "window.location.href") {
		t.Errorf("expected window.location.href in loading page, got: %s", body)
	}
	if !strings.Contains(body, "/api/services/") {
		t.Errorf("expected /api/services/ polling in loading page, got: %s", body)
	}
	if !strings.Contains(body, "Redirecting...") {
		t.Errorf("expected 'Redirecting...' in loading page, got: %s", body)
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
