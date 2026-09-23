package proxy

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

const StartupPath = "/__conslee_startup__"

const loadingHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
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
        .loading-steps {
            list-style: none;
            padding: 0;
            margin: 12px 0 0 0;
            font-size: 13px;
            color: #6b7280;
            line-height: 1.8;
        }
        .loading-step {
            position: relative;
            padding: 4px 0 4px 28px;
            transition: color 0.3s ease;
        }
        .loading-step::before {
            content: '';
            position: absolute;
            left: 0;
            top: 50%;
            transform: translateY(-50%);
            width: 18px;
            height: 18px;
            border: 2px solid #4b5563;
            border-radius: 50%;
            background: transparent;
            transition: border-color 0.3s ease, background-color 0.3s ease;
        }
        .loading-step-active {
            color: #f9fafb;
            font-weight: 500;
        }
        .loading-step-active::before {
            border-color: #7c3aed;
            background: radial-gradient(circle, #7c3aed 30%, transparent 31%);
            animation: loading-step-pulse 1.5s ease-in-out infinite;
        }
        .loading-step-completed {
            color: #10b981;
        }
        .loading-step-completed::before {
            border-color: #10b981;
            background: #10b981;
        }
        @keyframes loading-spin {
            to { transform: rotate(360deg); }
        }
        @keyframes loading-pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.7; }
        }
        @keyframes loading-step-pulse {
            0%, 100% { box-shadow: 0 0 0 0 rgba(124, 58, 237, 0.4); }
            50% { box-shadow: 0 0 0 6px rgba(124, 58, 237, 0); }
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
        <ol class="loading-steps">
            <li class="loading-step loading-step-active" id="step-starting">Starting service...</li>
            <li class="loading-step" id="step-waiting">Waiting service...</li>
            <li class="loading-step" id="step-redirecting">Redirecting...</li>
        </ol>
    </div>
    <script>
    (function() {
        var params = new URLSearchParams(window.location.search);
        var originalPath = params.get('path') || '/';
        var serviceName = '{{ .ServiceName }}';
        var stepStarting = document.getElementById('step-starting');
        var stepWaiting = document.getElementById('step-waiting');
        var stepRedirecting = document.getElementById('step-redirecting');
        function updateStep(activeEl) {
            var steps = [stepStarting, stepWaiting, stepRedirecting];
            for (var i = 0; i < steps.length; i++) {
                steps[i].classList.remove('loading-step-active');
                if (steps[i] === activeEl) {
                    steps[i].classList.add('loading-step-active');
                } else if (i < steps.indexOf(activeEl)) {
                    steps[i].classList.add('loading-step-completed');
                } else {
                    steps[i].classList.remove('loading-step-completed');
                }
            }
        }
        function poll() {
            fetch('/api/services')
            .then(function(r) { return r.json(); })
            .then(function(services) {
                if (!Array.isArray(services) || services.length === 0) {
                    updateStep(stepStarting);
                    return;
                }
                var svc = null;
                for (var i = 0; i < services.length; i++) {
                    if (services[i].name === serviceName) {
                        svc = services[i];
                        break;
                    }
                }
                if (!svc) {
                    updateStep(stepStarting);
                    return;
                }
                var url = "https://"+svc.host+svc.healthPath;
                var body = {url: url, requireSignature: true, allowWake: true};
                if (svc.customHeaders) {
                    try { body.customHeaders = svc.customHeaders; } catch(e) {}
                }
                fetch('/api/probes', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify(body)
                })
                .then(function(r) { return r.json(); })
                .then(function(result) {
                    if (result.status === 'healthy' && result.statusCode === 200) {
                        updateStep(stepRedirecting);
                        window.location.href = originalPath;
                    } else if (result.error && (result.error.indexOf('connection') !== -1 || result.error.indexOf('refused') !== -1)) {
                        updateStep(stepStarting);
                    } else {
                        updateStep(stepWaiting);
                    }
                })
                .catch(function() {
                    updateStep(stepStarting);
                });
            })
            .catch(function() {
                updateStep(stepStarting);
            });
        }
        setInterval(poll, 2000);
    })();
    </script>
</body>
</html>
`

type loadingPageData struct {
	ServiceName   string
	Message       string
}

func serveLoadingPage(w http.ResponseWriter, r *http.Request, svc *ServiceState, message string) {
	serviceName := "Service"
	if svc != nil {
		serviceName = svc.Config.Name
	}

	if message == "" {
		message = fmt.Sprintf("Container for %s is starting, please wait...", serviceName)
	}

	data := loadingPageData{
		ServiceName: serviceName,
		Message:     message,
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

// HandleStartupPage serves the loading page for the /__conslee_startup__ path.
// The original path is read from query parameters.
func (c *Conslee) HandleStartupPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != StartupPath {
		http.NotFound(w, r)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	serviceName := r.URL.Query().Get("service")

	var svc *ServiceState
	if serviceName != "" {
		if s, ok := c.reg.GetByName(serviceName); ok {
			svc = s
		}
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	serveLoadingPage(w, r, svc, "")
}