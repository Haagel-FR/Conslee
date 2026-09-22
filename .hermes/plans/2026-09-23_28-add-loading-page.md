# Plan: Add Loading Page for Container Startup (Issue #28)

## Goal
ブラウザからサービスにアクセスした時、コンテナが停止中であれば Conslee のローディング画面を表示し、バックグラウンドでコンテナを起動して、起動完了後に自動リロードする。

## Current Context / Assumptions
- Branch `28-add-loading-pages` is checked out, based on latest `origin/main`.
- `internal/proxy/reverse_proxy.go` の `ServeHTTP` で `ensureRunning` がエラー時に `502 backend unavailable` を返している（行118-122）。
- `ensureRunning` はコンテナ起動し、`waitTCP`/`waitHTTP` で準備完了までブロックする（行18-72）。
- 現在はAPIリクエスト等でも単に502が返されるだけで、ユーザーに何が起きているか伝わらない。
- フロントエンド（React UI）のローディングとは別物。バックエンドのリバースプロキシレイヤーの話。
- Stack: Go バックエンド + React フロントエンド。テストフレームワーク: Go testing + httptest。

## Architecture / Proposed Approach
1. `ServeHTTP` でブラウザリクエスト（Accept: text/html）かつサービスがまだ動いてない場合、HTMLローディングページを返す。
2. バックグラウンドで `ensureRunning` を開始し、完了を待たずにレスポンスを返す。
3. ローディングページには `<meta http-equiv="refresh" content="3">` で自動リロードを仕込む。
4. APIリクエスト等は従来通り `ensureRunning` を同期的に呼び、エラー時は502を返す。

---

## Step-by-Step Tasks

### Task 1: Create Loading Page HTML Template
**File:** `internal/proxy/loading_page.go` (NEW)

```go
package proxy

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

const loadingHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="refresh" content="3;url={{ .Path }}">
    <title>Starting {{ .ServiceName }}...</title>
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #050510;
            color: #f9fafb;
            font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        }
        .loading-container {
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 20px;
            text-align: center;
        }
        .loading-logo {
            width: 80px;
            height: auto;
            filter: drop-shadow(0 0 16px rgba(168, 85, 247, 0.5));
            animation: loading-pulse 2s ease-in-out infinite;
        }
        .loading-title {
            font-size: 18px;
            font-weight: 600;
        }
        .loading-message {
            font-size: 14px;
            color: #9ca3af;
        }
        .loading-spinner {
            width: 40px;
            height: 40px;
            border: 3px solid rgba(148, 163, 184, 0.3);
            border-top-color: #7c3aed;
            border-radius: 50%;
            animation: loading-spin 0.8s linear infinite;
        }
        @keyframes loading-spin {
            to { transform: rotate(360deg); }
        }
        @keyframes loading-pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.7; }
        }
    </style>
</head>
<body>
    <div class="loading-container">
        <svg class="loading-logo" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#7c3aed" stroke-width="2">
            <path d="M22 7.7c0-.6-.4-1.2-.8-1.5l-6.3-3.9a1.72 1.72 0 0 0-1.8 0L7.8 6.2c-.4.3-.8.9-.8 1.5v8.5c0 .6.4 1.2.8 1.5l6.3 3.9a1.72 1.72 0 0 0 1.8 0l6.3-3.9c.4-.3.8-.9.8-1.5Z"/>
            <path d="M12 22V12"/>
            <path d="m2 8 10 6 10-6"/>
            <path d="M7 5.1v6.4"/>
            <path d="M17 5.1v6.4"/>
        </svg>
        <h1 class="loading-title">Starting {{ .ServiceName }}</h1>
        <p class="loading-message">{{ .Message }}</p>
        <div class="loading-spinner"></div>
    </div>
</body>
</html>
`

type loadingPageData struct {
	ServiceName string
	Message     string
	Path        string
}

func serveLoadingPage(w http.ResponseWriter, r *http.Request, svc *ServiceState, message string) {
	if message == "" {
		message = fmt.Sprintf("Container for %s is starting, please wait...", svc.Config.Name)
	}

	path := r.URL.RequestURI()
	if path == "" {
		path = "/"
	}

	data := loadingPageData{
		ServiceName: svc.Config.Name,
		Message:     message,
		Path:        template.URLQueryEscaper(path),
	}

	tmpl, err := template.New("loading").Parse(loadingHTML)
	if err != nil {
		log.Printf("loading page template error: %v", err)
		http.Error(w, "loading", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("write loading page error: %v", err)
	}
}
```

**Verification:** `cd /opt/data/profiles/developer/Conslee && go build ./internal/proxy/` → No errors.

---

### Task 2: Add Helper Functions to reverse_proxy.go
**File:** `internal/proxy/reverse_proxy.go` (ADD before `ServeHTTP`)

```go
// isBrowserHTMLRequest returns true if the request is from a browser
// expecting an HTML response (vs API call, probe, etc.)
func isBrowserHTMLRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}
```

**Verification:** `go build ./...` compiles.

---

### Task 3: Modify ServeHTTP to Return Loading Page
**File:** `internal/proxy/reverse_proxy.go` (MODIFY `ServeHTTP`)

The `case ModeBoth, ModeOnDemand:` block should be changed from:

```go
	case ModeBoth, ModeOnDemand:
		if skipEnsure {
			w.Header().Set(probeSignatureHeader, svc.Config.Name)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := ensureRunning(r.Context(), c.rt, svc); err != nil {
			log.Printf("ensureRunning error for %s: %v", svc.Config.Name, err)
			http.Error(w, "backend unavailable", http.StatusBadGateway)
			return
		}
```

To:

```go
	case ModeBoth, ModeOnDemand:
		if skipEnsure {
			w.Header().Set(probeSignatureHeader, svc.Config.Name)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Show loading page for browser requests when container is not yet running
		if isBrowserHTMLRequest(r) && !c.isServiceReady(svc) {
			go ensureRunning(context.Background(), c.rt, svc)
			serveLoadingPage(w, r, svc, "")
			return
		}

		if err := ensureRunning(r.Context(), c.rt, svc); err != nil {
			log.Printf("ensureRunning error for %s: %v", svc.Config.Name, err)
			http.Error(w, "backend unavailable", http.StatusBadGateway)
			return
		}
```

Also add `isServiceReady` method to Conslee (in the same file, after ServeHTTP):

```go
// isServiceReady checks if the service's containers are already running
func (c *Conslee) isServiceReady(svc *ServiceState) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	names := svc.Config.Containers
	if len(names) == 0 && svc.Config.ContainerName != "" {
		names = []string{svc.Config.ContainerName}
	}

	for _, name := range names {
		st, err := c.rt.Inspect(ctx, name)
		if err != nil {
			continue
		}
		if st.Running {
			return true
		}
	}
	return false
}
```

**Verification:** `go build ./...` compiles.

---

### Task 4: Write Loading Page Tests
**File:** `internal/proxy/loading_page_test.go` (NEW)

```go
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
	if !strings.Contains(body, `meta http-equiv="refresh"`) {
		t.Errorf("expected meta refresh in loading page, got: %s", body)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content type, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestLoadingPage_NotShownForAPIRequest(t *testing.T) {
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

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	req.Host = "webapp.local"

	c.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "loading-spinner") {
		t.Errorf("expected spinner in loading page, got: %s", body)
	}
	if !strings.Contains(body, "loading-container") {
		t.Errorf("expected loading-container in loading page, got: %s", body)
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
```

**Verification:** `cd /opt/data/profiles/developer/Conslee && go test ./internal/proxy/ -v -run "TestLoading|TestIsBrowser|TestIsService" -count=1` → All tests pass.

---

### Task 5: Run All Tests
```bash
cd /opt/data/profiles/developer/Conslee && go test ./... -count=1
```

**Expected output:** All tests pass (including existing ones).

---

### Task 6: Commit
```bash
git add -A
git commit -m "feat: show loading page during container startup (closes #28)

- Add loading_page.go with HTML template and serveLoadingPage()
- Modify ServeHTTP to return loading page for browser requests when container is not running
- Container startup continues in background while user sees loading page
- Auto-refresh via meta tag reloads page after 3 seconds
- API requests (non-HTML) still get 502 backend unavailable on failure"
```

---

### Task 7: Push and Create PR
```bash
git push origin 28-add-loading-pages
gh pr create --base main --head 28-add-loading-pages \
  --title "feat: show loading page during container startup (closes #28)" \
  --body "## Summary
- Shows a loading page when a browser accesses a service whose containers are not running
- Container startup happens in background; page auto-refreshes every 3 seconds
- API/non-browser requests are unaffected (still get 502 on failure)
- HTML page uses Conslee's purple/dark theme with animated spinner"
```

---

### Task 8: Comment on Issue
```bash
gh issue comment 28 --body "Implemented! When you access a service whose containers are stopped, you'll now see a loading page with the Conslee logo and spinner. The page auto-refreshes every 3 seconds. Once the container is ready, the request proceeds normally. PR: #<number>"
```

---

## Tests / Validation

| Step | Command | Expected |
|------|---------|----------|
| 1 | `go test ./internal/proxy/ -v -run TestLoadingPage_ReturnsHTMLForBrowserRequest -count=1` | PASS |
| 2 | `go test ./internal/proxy/ -v -run TestLoadingPage_NotShownForAPIRequest -count=1` | PASS |
| 3 | `go test ./internal/proxy/ -v -run TestLoadingPage_NotShownWhenAlreadyRunning -count=1` | PASS |
| 4 | `go test ./internal/proxy/ -v -run TestLoadingPage_ContainsSpinner -count=1` | PASS |
| 5 | `go test ./internal/proxy/ -v -run TestIsBrowserHTMLRequest -count=1` | 5 subtests PASS |
| 6 | `go test ./internal/proxy/ -v -run TestIsServiceReady -count=1` | PASS |
| 7 | `go test ./... -count=1` | All tests pass |

---

## Risks, Tradeoffs, and Open Questions

| Risk | Mitigation |
|------|------------|
| Loading page loop if container fails to start | User retries → eventually 502. Future: add timeout/cancel. |
| `isServiceReady` adds Inspect call per request | Negligible for low-traffic proxy. Can cache later. |
| Background goroutine leaks | Goroutine exits when `ensureRunning` completes or `StartupTimeout` fires. |

**Open Questions:**
- Configurable refresh interval? → Keep simple at 3s.
- Show startup progress (e.g., "Step 2/3")? → Out of scope.

---

## Summary of Files Changed

| File | Action |
|------|--------|
| `internal/proxy/loading_page.go` | NEW |
| `internal/proxy/reverse_proxy.go` | MODIFY (add helpers + loading page branch) |
| `internal/proxy/loading_page_test.go` | NEW |
