package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxyHandler_IgnoresFavicon(t *testing.T) {
	req := httptest.NewRequest("GET", "/favicon.ico", nil)
	rr := httptest.NewRecorder()

	proxyHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProxyHandler_ForwardsRequest(t *testing.T) {
	// Start a mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "<status>")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<status><oat>55</oat></status>`))
	}))
	defer upstream.Close()

	// Simulate request to proxy
	xmlBody := []byte(`<status><oat>55</oat></status>`)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(xmlBody))
	req.Host = strings.TrimPrefix(upstream.URL, "http://")
	rr := httptest.NewRecorder()

	proxyHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "<status>")
}
