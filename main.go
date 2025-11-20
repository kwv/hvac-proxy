package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// dataDir is the directory to save XML files.
// It defaults to "/data" for the Docker container but can be
// overridden in tests.
var dataDir = "/data"

/* ---------------------- XML STRUCT ---------------------- */

type Status struct {
	XMLName   xml.Name `xml:"status"`
	LocalTime string   `xml:"localTime"`
	OAT       float64  `xml:"oat"`
	FiltrLvl  int      `xml:"filtrlvl"`

	IDU struct {
		CFM int `xml:"cfm"`
	} `xml:"idu"`

	Zones struct {
		Zones []Zone `xml:"zone"`
	} `xml:"zones"`
}

type Zone struct {
	ID               int     `xml:"id,attr"`
	CurrentTemp      float64 `xml:"rt"`
	RelativeHumidity int     `xml:"rh"`
	HeatSetPoint     float64 `xml:"htsp"`
	CoolSetPoint     float64 `xml:"clsp"`
}

/* ---------------------- METRICS ---------------------- */
type Metrics struct {
	mu sync.Mutex

	OutdoorAirTemp  float64
	FanCFM          int
	FilterLifePct   int
	IndoorTemp      float64
	IndoorRH        int
	HeatSP          float64
	CoolSP          float64
	LocalTimeParsed string
}

// Change metrics to be a pointer type
var metrics = &Metrics{}

func updateMetrics(s Status) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.OutdoorAirTemp = s.OAT
	metrics.FanCFM = s.IDU.CFM
	metrics.FilterLifePct = s.FiltrLvl

	if len(s.Zones.Zones) > 0 {
		z := s.Zones.Zones[0]
		metrics.IndoorTemp = z.CurrentTemp
		metrics.IndoorRH = z.RelativeHumidity
		metrics.HeatSP = z.HeatSetPoint
		metrics.CoolSP = z.CoolSetPoint
	}

	// Parse localTime safely
	t, err := time.Parse(time.RFC3339, s.LocalTime)
	if err == nil {
		metrics.LocalTimeParsed = t.Format("20060102150405")
	}
}

/* ---------------------- LOGGING ---------------------- */

func logRequest(r *http.Request, body []byte) {
	fullURL := fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.Host, r.RequestURI)
	log.Printf("[REQ]  %s %s (%d bytes)",
		r.Method, fullURL, len(body))
}

/* ---------------------- HVAC XML DETECTION ---------------------- */

func isHVACXML(b []byte) bool {
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "<status") {
		return true
	}
	if strings.HasPrefix(s, "<?xml") && strings.Contains(s, "<status") {
		return true
	}
	return false
}

func sanitizeString(tag string) string {
	// Replace unsafe filename characters
	tag = strings.TrimSpace(tag)
	tag = strings.ReplaceAll(tag, "/", "_")
	tag = strings.ReplaceAll(tag, "\\", "_")
	tag = strings.ReplaceAll(tag, ":", "_")
	tag = strings.ReplaceAll(tag, "*", "_")
	tag = strings.ReplaceAll(tag, "?", "_")
	tag = strings.ReplaceAll(tag, "\"", "_")
	tag = strings.ReplaceAll(tag, "<", "_")
	tag = strings.ReplaceAll(tag, ">", "_")
	tag = strings.ReplaceAll(tag, "|", "_")
	tag = strings.ReplaceAll(tag, "&", "_") // Also good to sanitize ampersands
	tag = strings.ReplaceAll(tag, "=", "_") // And equals signs
	return tag
}
func isXML(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				// Reached the end of the document without finding a complete element
				return false
			}
			return false
		}

		if _, ok := token.(xml.EndElement); ok {
			return true
		}
	}
}

/* ---------------------- PRETTIFY XML ---------------------- */

func prettifyXML(data []byte) []byte {
	// Check if it's XML content
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || (trimmed[0] != '<' && !bytes.HasPrefix(trimmed, []byte("<?xml"))) {
		return data
	}

	var buf bytes.Buffer
	decoder := xml.NewDecoder(bytes.NewReader(data))
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return data // Return original on error
		}
		if err := encoder.EncodeToken(token); err != nil {
			return data // Return original on error
		}
	}
	encoder.Flush()

	if buf.Len() > 0 {
		return buf.Bytes()
	}
	return data
}
func saveRequestBody(r *http.Request, body []byte) {
	// Process the body to get the XML data
	xmlData := body
	if bytes.HasPrefix(body, []byte("data=")) {
		encoded := bytes.TrimPrefix(body, []byte("data="))
		decoded, err := url.QueryUnescape(string(encoded))
		if err == nil {
			xmlData = []byte(decoded)
		}
	}

	// Check if it's HVAC XML and update metrics
	if isHVACXML(xmlData) {
		var status Status
		if err := xml.Unmarshal(xmlData, &status); err == nil {
			updateMetrics(status)
		} else {
			log.Printf("[HVAC] XML parse failed: %v", err)
		}
	}

	// Save the body to disk
	saveBody(r, body, "")
}

func saveResponseBody(r *http.Request, body []byte, _ int) {
	saveBody(r, body, "response")
}
func saveBody(r *http.Request, body []byte, suffix string) {
	if len(body) == 0 {
		return
	}

	content := body

	// Decode URL-encoded HVAC form
	if bytes.HasPrefix(content, []byte("data=")) {
		encoded := bytes.TrimPrefix(content, []byte("data="))
		if decoded, err := url.QueryUnescape(string(encoded)); err == nil {
			content = []byte(decoded)
		}
	}

	// Format XML nicely (no-op for non-XML)
	content = prettifyXML(content)

	var filename string
	// Construct filename using URL path
	path := r.RequestURI
	if path == "" {
		path = r.URL.Path
		// Re-add query string if it exists, as it's not part of URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
	}
	path = strings.TrimPrefix(path, "/") // Remove leading slash
	path = sanitizeString(path)

	filename = r.Method + "-" + path
	if suffix != "" {
		filename += "-" + suffix
	}
	if isXML(content) {
		filename += ".xml"
	}

	// Write file to dataDir (which is /data in Docker, or temp dir in tests)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create directory %s: %v", dataDir, err)
		return
	}

	filePath := filepath.Join(dataDir, filename)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		log.Printf("[ERROR] Failed to write file %s: %v", filePath, err)
		return
	}
}

/* ---------------------- PROXY ---------------------- */

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// Ignore favicon requests to avoid log spam
	if r.URL.Path == "/favicon.ico" {
		http.NotFound(w, r)
		return
	}

	startTime := time.Now()

	/* ---- READ INBOUND REQUEST BODY ---- */
	var reqBuf bytes.Buffer
	if r.Body != nil {
		io.Copy(&reqBuf, r.Body)
	}
	body := reqBuf.Bytes()

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	logRequest(r, body)
	saveRequestBody(r, body)

	/* ---- FORWARD REQUEST UPSTREAM ---- */
	// Build the upstream URL using the original Host header
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	targetURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	upReq, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create upstream request", 500)
		return
	}
	upReq.Header = r.Header.Clone()

	resp, err := http.DefaultClient.Do(upReq)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Printf("[RESP] %s %s → ERROR: %v (elapsed: %v)",
			r.Method, r.RequestURI, err, elapsed)
		http.Error(w, "Upstream error: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	log.Printf("[RESP] %s %s → %d (elapsed: %v)",
		r.Method, r.RequestURI, resp.StatusCode, elapsed)

	/* ---- READ AND SAVE RESPONSE BODY ---- */
	var respBuf bytes.Buffer
	io.Copy(&respBuf, resp.Body)
	respBody := respBuf.Bytes()

	// Check if updates should be blocked
	blockUpdates := os.Getenv("BLOCK_UPDATES") == "true"
	if blockUpdates {
		// Regex to match all <update> blocks
		re := regexp.MustCompile(`<update[^>]*>.*?</update>`)
		respBody = re.ReplaceAll(respBody, []byte{})
	}

	// Save response body (for GET requests or any response)
	saveResponseBody(r, respBody, resp.StatusCode)

	/* ---- PASS THROUGH RESPONSE ---- */
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

/* ---------------------- METRICS ---------------------- */

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	metrics.mu.Lock()
	// Create a local snapshot of the metrics struct *while locked*
	// This prevents a race condition where metrics are read while being written
	m := *metrics
	metrics.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain")

	fmt.Fprintf(w, "# HELP outdoorAirTemp degrees in F\n")
	fmt.Fprintf(w, "# TYPE outdoorAirTemp gauge\n")
	fmt.Fprintf(w, "outdoorAirTemp %.1f\n", m.OutdoorAirTemp)

	fmt.Fprintf(w, "# HELP fanSpeed cubic feet minute\n")
	fmt.Fprintf(w, "# TYPE fanSpeed gauge\n")
	fmt.Fprintf(w, "fanSpeed %d\n", m.FanCFM)

	fmt.Fprintf(w, "# HELP Stage StageName\n")
	fmt.Fprintf(w, "# TYPE Stage gauge\n")
	fmt.Fprintf(w, "Stage 0\n")

	fmt.Fprintf(w, "# HELP filter %% of filter life\n")
	fmt.Fprintf(w, "# TYPE filter gauge\n")
	fmt.Fprintf(w, "filter %d\n", m.FilterLifePct)

	fmt.Fprintf(w, "# HELP temperature indoor temp\n")
	fmt.Fprintf(w, "# TYPE temperature gauge\n")
	fmt.Fprintf(w, "temperature %.1f\n", m.IndoorTemp)

	fmt.Fprintf(w, "# HELP relativeHumidity indoor relative humidity\n")
	fmt.Fprintf(w, "# TYPE relativeHumidity gauge\n")
	fmt.Fprintf(w, "relativeHumidity %d\n", m.IndoorRH)

	fmt.Fprintf(w, "# HELP heatSetPoint heat set point\n")
	fmt.Fprintf(w, "# TYPE heatSetPoint gauge\n")
	fmt.Fprintf(w, "heatSetPoint %.1f\n", m.HeatSP)

	fmt.Fprintf(w, "# HELP coolingSetPoint cooling set point\n")
	fmt.Fprintf(w, "# TYPE coolingSetPoint gauge\n")
	fmt.Fprintf(w, "coolingSetPoint %.1f\n", m.CoolSP)

	fmt.Fprintf(w, "# HELP localtime last refreshed time\n")
	fmt.Fprintf(w, "# TYPE localtime gauge\n")
	val, _ := strconv.Atoi(m.LocalTimeParsed)
	fmt.Fprintf(w, "localtime %d\n", val)
}

/* ---------------------- MAIN ---------------------- */

func main() {
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/", proxyHandler)

	log.Println("hvac-proxy listening on :8080")
	log.Printf("Saving XML files to %s/", dataDir)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
