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
    <script>
    (function() {
        var originalPath = "{{ .OriginalPath }}";
        var serviceName = "{{ .ServiceName }}";
        var titleEl = document.querySelector('.loading-title');
        function poll() {
            fetch('/api/services/')
                .then(function(r) { return r.json(); })
                .then(function(services) {
                    for (var i = 0; i < services.length; i++) {
                        if (services[i].name === serviceName && services[i].running === true) {
                            titleEl.textContent = 'Redirecting...';
                            window.location.href = originalPath;
                            return;
                        }
                    }
                })
                .catch(function() {});
        }
        setInterval(poll, 2000);
    })();
    </script>
</body>
</html>
`

type loadingPageData struct {
	ServiceName  string
	Message      string
	Path         string
	OriginalPath string
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
		ServiceName:  svc.Config.Name,
		Message:      message,
		Path:         template.URLQueryEscaper(path),
		OriginalPath: template.JSEscaper(path),
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
