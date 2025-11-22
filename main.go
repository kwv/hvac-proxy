package main

import (
	"bytes"
	"fmt"
	"hvac-proxy/hvac"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func logRequest(r *http.Request, body []byte) {
	// Infer scheme from the connection
	var scheme string
	if r.TLS != nil {
		scheme = "https"
	} else {
		scheme = "http"
	}

	// Build full URL using inferred scheme
	fullURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)
	log.Printf("[REQ]  %s %s → (%d bytes)", r.Method, fullURL, len(body))
}

func logResponse(resp *http.Response, elapsed time.Duration) {
	// Use the Request field from the response to get URL details
	fullURL := fmt.Sprintf("%s://%s%s",
		resp.Request.URL.Scheme,
		resp.Request.Host,
		resp.Request.URL.RequestURI())
	log.Printf("[RESP] %s %s → %d (elapsed: %v)", resp.Request.Method, fullURL, resp.StatusCode, elapsed)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// Ignore favicon requests
	if r.URL.Path == "/favicon.ico" {
		http.NotFound(w, r)
		return
	}

	// Read request body
	var reqBuf bytes.Buffer
	if r.Body != nil {
		io.Copy(&reqBuf, r.Body)
	}
	body := reqBuf.Bytes()
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	logRequest(r, body)
	hvac.SaveBody(r, body, true)

	// Forward to upstream
	targetURL := fmt.Sprintf("http://%s%s", r.Host, r.RequestURI)

	upReq, _ := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
	upReq.Header = r.Header.Clone()
	startTime := time.Now()
	resp, err := http.DefaultClient.Do(upReq)
	if err != nil {
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	elapsed := time.Since(startTime)
	// Log and save response body
	var respBuf bytes.Buffer
	io.Copy(&respBuf, resp.Body)
	respBody := respBuf.Bytes()

	logResponse(resp, elapsed)
	hvac.SaveBody(r, respBody, false)

	// Write response
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func init() {
	// Determine DATA_DIR or fallback to temp dir
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		tmp, err := os.MkdirTemp("", "hvac-data-*")
		if err != nil {
			fmt.Printf("Failed to create temp directory: %v\n", err)
			return
		}
		os.Setenv("DATA_DIR", tmp)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	os.Setenv("PORT", port)
}
func main() {

	http.HandleFunc("/", proxyHandler)
	http.HandleFunc("/metrics", hvac.HandleMetrics)

	fmt.Printf("Server running on port %s\n saving to %s\n",
		os.Getenv("PORT"), os.Getenv("DATA_DIR"))
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
