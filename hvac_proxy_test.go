package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

/* ---------------------- HELPER FUNCTIONS ---------------------- */

// setupTestDir is no longer needed as we override the global 'dataDir'
// in each test function that needs to write files.

func createTestStatus() Status {
	return Status{
		LocalTime: time.Now().Format(time.RFC3339),
		OAT:       65.5,
		FiltrLvl:  75,
		IDU: struct {
			CFM int `xml:"cfm"`
		}{CFM: 500},
		Zones: struct {
			Zones []Zone `xml:"zone"`
		}{
			Zones: []Zone{
				{
					ID:               1,
					CurrentTemp:      72.0,
					RelativeHumidity: 45,
					HeatSetPoint:     68.0,
					CoolSetPoint:     74.0,
				},
			},
		},
	}
}

/* ---------------------- UNIT TESTS ---------------------- */

func TestIsHVACXML(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "status tag",
			input:    []byte("<status version=\"1.0\">"),
			expected: true,
		},
		{
			name:     "xml declaration with status",
			input:    []byte("<?xml version=\"1.0\"?><status>"),
			expected: true,
		},
		{
			name:     "not hvac xml",
			input:    []byte("<config>"),
			expected: false,
		},
		{
			name:     "empty",
			input:    []byte(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHVACXML(tt.input)
			if result != tt.expected {
				t.Errorf("isHVACXML() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPrettifyXML(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		contains string
	}{
		{
			name:     "compact xml",
			input:    []byte("<root><child>value</child></root>"),
			contains: "  <child>",
		},
		{
			name:     "not xml",
			input:    []byte("not xml"),
			contains: "not xml",
		},
		{
			name:     "empty",
			input:    []byte(""),
			contains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prettifyXML(tt.input)
			if !bytes.Contains(result, []byte(tt.contains)) {
				t.Errorf("prettifyXML() result doesn't contain %q", tt.contains)
			}
		})
	}
}

func TestUpdateMetrics(t *testing.T) {
	status := createTestStatus()
	updateMetrics(status)

	// FIX: Must lock metrics to prevent race condition during read
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	if metrics.OutdoorAirTemp != 65.5 {
		t.Errorf("OutdoorAirTemp = %v, want 65.5", metrics.OutdoorAirTemp)
	}
	if metrics.FanCFM != 500 {
		t.Errorf("FanCFM = %v, want 500", metrics.FanCFM)
	}
	if metrics.FilterLifePct != 75 {
		t.Errorf("FilterLifePct = %v, want 75", metrics.FilterLifePct)
	}
	if metrics.IndoorTemp != 72.0 {
		t.Errorf("IndoorTemp = %v, want 72.0", metrics.IndoorTemp)
	}
	if metrics.IndoorRH != 45 {
		t.Errorf("IndoorRH = %v, want 45", metrics.IndoorRH)
	}
	if metrics.HeatSP != 68.0 {
		t.Errorf("HeatSP = %v, want 68.0", metrics.HeatSP)
	}
	if metrics.CoolSP != 74.0 {
		t.Errorf("CoolSP = %v, want 74.0", metrics.CoolSP)
	}
}

/* ---------------------- HANDLER TESTS ---------------------- */

func TestMetricsHandler(t *testing.T) {
	// Set up test metrics
	status := createTestStatus()
	updateMetrics(status)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	metricsHandler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Content-Type = %v, want text/plain", contentType)
	}

	expectedStrings := []string{
		"# HELP outdoorAirTemp",
		"# TYPE outdoorAirTemp gauge",
		"outdoorAirTemp 65.5",
		"fanSpeed 500",
		"filter 75",
		"temperature 72.0",
		"relativeHumidity 45",
		"heatSetPoint 68.0",
		"coolingSetPoint 74.0",
	}

	bodyStr := string(body)
	for _, expected := range expectedStrings {
		if !strings.Contains(bodyStr, expected) {
			t.Errorf("Response body doesn't contain %q", expected)
		}
	}
}

func TestProxyHandlerWithMockUpstream(t *testing.T) {
	// FIX: Override dataDir to a temp dir for this test
	tmpDir := t.TempDir()
	oldDataDir := dataDir
	dataDir = tmpDir
	defer func() { dataDir = oldDataDir }()

	// Create a mock upstream server
	upstreamResponse := []byte("<response>OK</response>")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(upstreamResponse)
	}))
	defer upstream.Close()

	// Parse upstream URL
	upstreamURL, _ := url.Parse(upstream.URL)

	// Create test status XML
	statusXML := `<status version="1.0">
		<localTime>2025-11-15T18:52:45-05:00</localTime>
		<oat>63</oat>
		<filtrlvl>40</filtrlvl>
		<idu><cfm>437</cfm></idu>
		<zones><zone id="1"><rt>71.0</rt><rh>49</rh><htsp>68.0</htsp><clsp>72.0</clsp></zone></zones>
	</status>`

	// URL encode the status
	formData := "data=" + url.QueryEscape(statusXML)

	// Create request to proxy
	req := httptest.NewRequest("POST", "/status", strings.NewReader(formData))
	req.Host = upstreamURL.Host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	proxyHandler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	if !bytes.Equal(body, upstreamResponse) {
		t.Errorf("Response body = %q, want %q", body, upstreamResponse)
	}

	// Verify metrics were updated
	// FIX: Must lock metrics to prevent race condition
	metrics.mu.Lock()
	if metrics.OutdoorAirTemp != 63.0 {
		t.Errorf("OutdoorAirTemp = %v, want 63.0", metrics.OutdoorAirTemp)
	}
	if metrics.FanCFM != 437 {
		t.Errorf("FanCFM = %v, want 437", metrics.FanCFM)
	}
	metrics.mu.Unlock()

	// FIX: Verify files were saved
	expectedReqFile := filepath.Join(tmpDir, "POST-status.xml")
	if _, err := os.Stat(expectedReqFile); os.IsNotExist(err) {
		t.Errorf("proxyHandler did not create expected request file: %s", expectedReqFile)
	}

	expectedRespFile := filepath.Join(tmpDir, "POST-status-response.xml")
	if _, err := os.Stat(expectedRespFile); os.IsNotExist(err) {
		t.Errorf("proxyHandler did not create expected response file: %s", expectedRespFile)
	}
}

func TestProxyHandlerURLDecoding(t *testing.T) {
	// FIX: Override dataDir to avoid writing to /data during tests
	tmpDir := t.TempDir()
	oldDataDir := dataDir
	dataDir = tmpDir
	defer func() { dataDir = oldDataDir }()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	tests := []struct {
		name        string
		body        string
		contentType string
	}{
		{
			name:        "url encoded form data",
			body:        "data=%3Cstatus%3E%3Coat%3E50%3C%2Foat%3E%3C%2Fstatus%3E",
			contentType: "application/x-www-form-urlencoded",
		},
		{
			name:        "plain xml",
			body:        "<status><oat>50</oat></status>",
			contentType: "application/xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.body))
			req.Host = upstreamURL.Host
			req.Header.Set("Content-Type", tt.contentType)

			w := httptest.NewRecorder()
			proxyHandler(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
			}
		})
	}
}

// FIX: This test is now a real assertion, not a stub
func TestFilenameConstruction(t *testing.T) {
	// FIX: Override dataDir to a temp dir for this test
	tmpDir := t.TempDir()
	oldDataDir := dataDir
	dataDir = tmpDir
	defer func() { dataDir = oldDataDir }()

	req, _ := http.NewRequest("POST", "/user/123/profile", nil)
	body := []byte("<root>test<sub>content</sub></root>")
	suffix := "response" // Use a real suffix

	saveBody(req, body, suffix)

	// Verify the file was created with the correct name
	expectedFilename := "POST-user_123_profile-response.xml"
	expectedFilePath := filepath.Join(tmpDir, expectedFilename)

	if _, err := os.Stat(expectedFilePath); os.IsNotExist(err) {
		t.Errorf("saveBody() did not create file %q", expectedFilename)
		return
	}

	// Verify the file content was prettified
	content, err := os.ReadFile(expectedFilePath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	// FIX: The assertion was wrong. It should check for the indented *child content*.
	expectedContent := "<root>test\n  <sub>content</sub>\n</root>"
	if !bytes.Contains(content, []byte(expectedContent)) {
		t.Errorf("File content = %q, want to contain prettified XML (%q)", content, expectedContent)
	}
}

/* ---------------------- INTEGRATION TESTS ---------------------- */

func TestFullProxyFlow(t *testing.T) {
	// FIX: Override dataDir to a temp dir for this test
	tmpDir := t.TempDir()
	oldDataDir := dataDir
	dataDir = tmpDir
	defer func() { dataDir = oldDataDir }()

	// Create a full upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request was forwarded correctly
		body, _ := io.ReadAll(r.Body)
		if !bytes.Contains(body, []byte("data=")) {
			t.Error("Request body not forwarded correctly")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<response>success</response>"))
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	// Create realistic HVAC status
	status := createTestStatus()
	statusXML, _ := xml.Marshal(status)
	formData := "data=" + url.QueryEscape(string(statusXML))

	// Send request through proxy
	req := httptest.NewRequest("POST", "/status", strings.NewReader(formData))
	req.Host = upstreamURL.Host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	proxyHandler(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "success") {
		t.Errorf("Response body = %q, want to contain 'success'", body)
	}

	// Verify metrics were updated
	// FIX: Must lock metrics to prevent race condition
	metrics.mu.Lock()
	if metrics.FilterLifePct != 75 {
		t.Errorf("FilterLifePct = %v, want 75", metrics.FilterLifePct)
	}
	metrics.mu.Unlock()

	// FIX: Verify files were saved
	reqFile := filepath.Join(tmpDir, "POST-status.xml")
	if _, err := os.Stat(reqFile); os.IsNotExist(err) {
		t.Errorf("proxyHandler did not create request file: %s", reqFile)
	}

	respFile := filepath.Join(tmpDir, "POST-status-response.xml")
	if _, err := os.Stat(respFile); os.IsNotExist(err) {
		t.Errorf("proxyHandler did not create response file: %s", respFile)
	}
}

/* ---------------------- BENCHMARK TESTS ---------------------- */

func BenchmarkPrettifyXML(b *testing.B) {
	xml := []byte("<root><child>value</child><child2>value2</child2></root>")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prettifyXML(xml)
	}
}

func BenchmarkUpdateMetrics(b *testing.B) {
	status := createTestStatus()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updateMetrics(status)
	}
}
