package main

import (
	"html"
)

const defaultColdStartMessage = "Service is starting, please wait..."

// generateWarmingPageHTML creates an HTML page to display during cold starts.
// The page includes:
// - Custom message (or default if empty)
// - Auto-refresh every 5 seconds
// - Loading animation
// - Professional styling
func generateWarmingPageHTML(customMessage string) string {
	message := customMessage
	if message == "" {
		message = defaultColdStartMessage
	}
	
	// Escape HTML to prevent XSS
	safeMessage := html.EscapeString(message)
	
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="refresh" content="5">
    <title>Service Starting</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            color: #fff;
        }
        .container {
            text-align: center;
            padding: 2rem;
            max-width: 600px;
        }
        h1 {
            font-size: 2.5rem;
            margin-bottom: 1rem;
            font-weight: 600;
        }
        .message {
            font-size: 1.2rem;
            margin-bottom: 2rem;
            opacity: 0.9;
            white-space: pre-wrap;
        }
        .spinner {
            width: 60px;
            height: 60px;
            border: 4px solid rgba(255, 255, 255, 0.3);
            border-top-color: #fff;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 2rem;
        }
        @keyframes spin {
            to { transform: rotate(360deg); }
        }
        .info {
            font-size: 0.9rem;
            opacity: 0.7;
            margin-top: 2rem;
        }
        .powered-by {
            font-size: 0.8rem;
            opacity: 0.5;
            margin-top: 3rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="spinner"></div>
        <h1>Service Starting</h1>
        <div class="message">` + safeMessage + `</div>
        <p class="info">This page will automatically refresh...</p>
        <p class="powered-by">Powered by KEDA HTTP Add-on</p>
    </div>
</body>
</html>`
}
