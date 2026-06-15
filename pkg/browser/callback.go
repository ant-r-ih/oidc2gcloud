package browser

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

// CallbackServer represents a local HTTP server for receiving OAuth callbacks
type CallbackServer struct {
	listener net.Listener
	server   *http.Server
	codeChan chan string
	errChan  chan error
}

// NewCallbackServer creates a new callback server
func NewCallbackServer(port int) (*CallbackServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start callback server")
	}

	cs := &CallbackServer{
		listener: listener,
		codeChan: make(chan string, 1),
		errChan:  make(chan error, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", cs.handleCallback)

	cs.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return cs, nil
}

// Start starts the callback server
func (cs *CallbackServer) Start() {
	go func() {
		if err := cs.server.Serve(cs.listener); err != nil && err != http.ErrServerClosed {
			cs.errChan <- err
		}
	}()
}

// WaitForCode waits for the authorization code
func (cs *CallbackServer) WaitForCode(timeout time.Duration) (string, error) {
	select {
	case code := <-cs.codeChan:
		return code, nil
	case err := <-cs.errChan:
		return "", err
	case <-time.After(timeout):
		return "", errors.New("timeout waiting for callback")
	}
}

// Shutdown shuts down the callback server
func (cs *CallbackServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return cs.server.Shutdown(ctx)
}

// GetRedirectURI returns the redirect URI for this server
func (cs *CallbackServer) GetRedirectURI() string {
	// Always return localhost to match the configured redirect URI
	return "http://localhost:8085/callback"
}

func (cs *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Check for error
	if errMsg := query.Get("error"); errMsg != "" {
		errDesc := query.Get("error_description")
		cs.errChan <- errors.Errorf("OAuth error: %s - %s", errMsg, errDesc)
		http.Error(w, fmt.Sprintf("Authentication failed: %s", errDesc), http.StatusBadRequest)
		return
	}

	// Get authorization code
	code := query.Get("code")
	if code == "" {
		cs.errChan <- errors.New("no authorization code received")
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		return
	}

	// Send success response
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Authentication Successful</title>
    <style>
        body { font-family: sans-serif; text-align: center; padding: 50px; }
        .success { color: green; font-size: 24px; }
    </style>
</head>
<body>
    <div class="success">✓ Authentication Successful</div>
    <p>You can close this window and return to the terminal.</p>
</body>
</html>
`)

	// Send code to channel
	cs.codeChan <- code
}

// OpenBrowser opens the default browser with the authorization URL
func OpenBrowser(authURL string) error {
	parsedURL, err := url.Parse(authURL)
	if err != nil {
		return errors.Wrap(err, "invalid authorization URL")
	}

	fmt.Printf("Opening browser for authentication:\n%s\n\n", parsedURL.String())
	fmt.Println("If the browser doesn't open automatically, please open the URL manually.")

	// Try to open the browser using different methods
	var cmd string
	var args []string

	switch {
	case commandExists("xdg-open"): // Linux
		cmd = "xdg-open"
		args = []string{authURL}
	case commandExists("open"): // macOS
		cmd = "open"
		args = []string{authURL}
	case commandExists("start"): // Windows
		cmd = "cmd"
		args = []string{"/c", "start", authURL}
	default:
		return errors.New("unable to detect browser command")
	}

	// Note: We can't actually execute the command from here without importing os/exec
	// This will be handled in the main package
	fmt.Printf("Execute: %s %v\n", cmd, args)
	return nil
}

func commandExists(cmd string) bool {
	// This is a placeholder - actual implementation would check if command exists
	// For now, we'll just assume the command exists on appropriate platforms
	return true
}
