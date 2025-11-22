package hvac

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// This file contains functions to parse HVAC status XML data and generate
// Prometheus-formatted metrics, which are saved to disk.
// It also includes the HTTP handler for the "/metrics" endpoint.

// XML STRUCTURES
// These structs represent the XML data structure returned by the HVAC system.

// IDU represents the Indoor Unit data in the XML.
type IDU struct {
	CFM    int    `xml:"cfm"`    // Fan speed in cubic feet per minute
	OPSTAT string `xml:"opstat"` // Operation status of the unit
}

// Zones represents the collection of zones in the HVAC system.
type Zones struct {
	Zones []Zone `xml:"zone"` // List of individual zone data
}

// Zone represents a specific zone in the HVAC system.
type Zone struct {
	ID               int     `xml:"id,attr"` // Zone ID
	CurrentTemp      float64 `xml:"rt"`      // Current temperature in the zone
	RelativeHumidity int     `xml:"rh"`      // Relative humidity in the zone
	HeatSetPoint     float64 `xml:"htsp"`    // Heating set point temperature
	CoolSetPoint     float64 `xml:"clsp"`    // Cooling set point temperature
}

// Status represents the overall status of the HVAC system.
type Status struct {
	XMLName   xml.Name `xml:"status"`    // Root XML element
	LocalTime string   `xml:"localTime"` // Local time from the system
	OAT       float64  `xml:"oat"`       // Outdoor air temperature in Fahrenheit
	FiltrLvl  int      `xml:"filtrlvl"`  // Filter life percentage
	IDU       IDU      `xml:"idu"`       // Indoor Unit data
	Zones     Zones    `xml:"zones"`     // Zones data
}

// SaveMetricsFromXML parses the given XML data and saves Prometheus-formatted metrics to a file.
func SaveMetricsFromXML(xmlData []byte) error {
	s := strings.TrimSpace(string(xmlData))
	if !strings.HasPrefix(s, "<status") {
		return fmt.Errorf("not HVAC status XML")
	}

	var status Status
	if err := xml.Unmarshal(xmlData, &status); err != nil {
		return fmt.Errorf("failed to unmarshal XML: %w", err)
	}

	prometheusStr := status.ToPrometheus()

	filePath := filepath.Join(os.Getenv("DATA_DIR"), "metrics_last.txt")
	if err := os.WriteFile(filePath, []byte(prometheusStr), 0644); err != nil {
		return fmt.Errorf("failed to save metrics to file: %w", err)
	}

	return nil
}

// ToPrometheus generates a Prometheus-formatted string directly from the Status data.
func (s *Status) ToPrometheus() string {
	var b strings.Builder

	// Outdoor Air Temperature
	b.WriteString("# HELP outdoorAirTemp degrees in F\n")
	b.WriteString("# TYPE outdoorAirTemp gauge\n")
	b.WriteString(fmt.Sprintf("outdoorAirTemp %.1f\n", s.OAT))

	// Fan Speed
	b.WriteString("# HELP fanSpeed cubic feet minute\n")
	b.WriteString("# TYPE fanSpeed gauge\n")
	b.WriteString(fmt.Sprintf("fanSpeed %d\n", s.IDU.CFM))

	// Operation Stage
	b.WriteString("# HELP Stage StageName\n")
	b.WriteString("# TYPE Stage gauge\n")
	b.WriteString(fmt.Sprintf("stage %s\n", s.IDU.OPSTAT))

	// Filter Life
	b.WriteString("# HELP filter percent of filter life\n")
	b.WriteString("# TYPE filter gauge\n")
	b.WriteString(fmt.Sprintf("filter %d\n", s.FiltrLvl))

	// Zone Temperature
	b.WriteString("# HELP temperature indoor temp\n")
	b.WriteString("# TYPE temperature gauge\n")
	b.WriteString(fmt.Sprintf("temperature %.1f\n", s.Zones.Zones[0].CurrentTemp))

	// Zone Relative Humidity
	b.WriteString("# HELP relativeHumidity indoor relative humidity\n")
	b.WriteString("# TYPE relativeHumidity gauge\n")
	b.WriteString(fmt.Sprintf("relativeHumidity %d\n", s.Zones.Zones[0].RelativeHumidity))

	// Zone Heat Set Point
	b.WriteString("# HELP heatSetPoint heat set point\n")
	b.WriteString("# TYPE heatSetPoint gauge\n")
	b.WriteString(fmt.Sprintf("heatSetPoint %.1f\n", s.Zones.Zones[0].HeatSetPoint))

	// Zone Cooling Set Point
	b.WriteString("# HELP coolingSetPoint cooling set point\n")
	b.WriteString("# TYPE coolingSetPoint gauge\n")
	b.WriteString(fmt.Sprintf("coolingSetPoint %.1f\n", s.Zones.Zones[0].CoolSetPoint))

	// Local Time
	b.WriteString("# HELP localtime last refreshed time\n")
	b.WriteString("# TYPE localtime gauge\n")

	// Attempt to parse local time using RFC3339 format
	t, err := time.Parse(time.RFC3339, s.LocalTime)
	if err != nil {
		// Fallback for non-standard time formats (e.g., with offset like -05:58)
		fixed := s.LocalTime
		if i := strings.LastIndex(fixed, ":"); i > len("2006-01-02T15:04:05") {
			fixed = fixed[:i] + fixed[i+1:]
		}
		layout := "2006-01-02T15:04:05-0700"
		t, err = time.Parse(layout, fixed)
	}

	if err == nil {
		// Convert time to a numeric format suitable for Prometheus (YYYYMMDDhhmmss)
		formatted := t.Format("20060102150405")
		if val, err := strconv.Atoi(formatted); err == nil {
			b.WriteString(fmt.Sprintf("localtime %d\n", val))
		} else {
			b.WriteString("localtime 0\n")
		}
	} else {
		b.WriteString("localtime 0\n")
	}

	return b.String()
}

// HandleMetrics is the HTTP handler for the "/metrics" endpoint.
// It reads the last saved metrics from disk and serves them as plain text.
func HandleMetrics(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join(os.Getenv("DATA_DIR"), "metrics_last.txt")

	// Read the metrics file
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Failed to read metrics file", http.StatusInternalServerError)
		return
	}

	// Set the content type to plain text and write the response
	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}
