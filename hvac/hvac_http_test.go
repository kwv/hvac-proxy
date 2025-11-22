package hvac_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"hvac-proxy/hvac"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveBody_FilenameConstruction verifies that the filename is constructed correctly.
func TestSaveBody_FilenameConstruction(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	body := []byte("<response>OK</response>")
	req, _ := http.NewRequest("POST", "/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	hvac.SaveBody(req, body, false)

	expectedFile := filepath.Join(tmpDir, "POST-status-response.xml")
	assert.FileExists(t, expectedFile)

	content, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "OK")
}

// TestSaveBody_URLDecoding verifies that URL-encoded data is decoded properly.
func TestSaveBody_URLDecoding(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	encodedBody := []byte("data=%3Cresponse%3EOK%3C%2Fresponse%3E")
	req, _ := http.NewRequest("GET", "/test", bytes.NewBuffer(encodedBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	hvac.SaveBody(req, encodedBody, true)

	expectedFile := filepath.Join(tmpDir, "GET-test.xml")
	assert.FileExists(t, expectedFile)

	content, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "<response>OK</response>")
}

// TestSaveBody_EmptyBody verifies that no file is written for empty body.
func TestSaveBody_EmptyBody(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	body := []byte{}
	req, _ := http.NewRequest("POST", "/empty", bytes.NewBuffer(body))

	hvac.SaveBody(req, body, false)

	expectedFile := filepath.Join(tmpDir, "POST-empty-empty.xml")
	assert.NoFileExists(t, expectedFile)
}

// TestSaveBody_MetricsUpdate verifies metrics are saved only for request bodies.
func TestSaveBody_MetricsUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	// Valid HVAC status XML
	body := []byte(`<status><localTime>2025-11-21T19:49:44-05:00</localTime><oat>72</oat><filtrlvl>90</filtrlvl><idu><cfm>100</cfm></idu><zones><zone id="1"><rt>70</rt><rh>40</rh><htsp>68</htsp><clsp>75</clsp></zone></zones></status>`)
	req, _ := http.NewRequest("POST", "/status", bytes.NewBuffer(body))

	// Request case should trigger metrics save
	hvac.SaveBody(req, body, true)
	metricsFile := filepath.Join(tmpDir, "metrics_last.txt")
	assert.FileExists(t, metricsFile)

	// Response case should NOT trigger metrics save
	os.Remove(metricsFile)
	hvac.SaveBody(req, body, false)
	assert.NoFileExists(t, metricsFile)
}

// TestSaveBody_BlockUpdates verifies that <update> blocks are stripped when BLOCK_UPDATES=true.
func TestSaveBody_BlockUpdates(t *testing.T) {

	tmpDir := t.TempDir()
	os.Setenv("DATA_DIR", tmpDir)
	os.Setenv("BLOCK_UPDATES", "true")
	defer func() {
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("BLOCK_UPDATES")
	}()

	body := []byte(`<updates xmlns="http://schema.ota.carrier.com" xmlns="http://schema.ota.carrier.com"><update xmlns="http://schema.ota.carrier.com"><type xmlns="http://schema.ota.carrier.com">thermostat</type><model xmlns="http://schema.ota.carrier.com">SYSTXCCITC01-A</model><locales xmlns="http://schema.ota.carrier.com"><locale xmlns="http://schema.ota.carrier.com">en-us</locale></locales><version xmlns="http://schema.ota.carrier.com">14.02</version><url xmlns="http://schema.ota.carrier.com">http://www.ota.ing.carrier.com/updates/systxccit-14.02.hex</url><releaseNotes xmlns="http://schema.ota.carrier.com"><url xmlns="http://schema.ota.carrier.com" type="text/plain" locale="en-us">http://www.ota.ing.carrier.com/releaseNotes/systxccit-14.02.txt</url><url xmlns="http://schema.ota.carrier.com" type="text/html" locale="en-us">http://www.ota.ing.carrier.com/releaseNotes/systxccit-14.02.html</url></releaseNotes></update></updates>`)

	req, _ := http.NewRequest("POST", "/strip", bytes.NewBuffer(body))

	hvac.SaveBody(req, body, true)

	expectedFile := filepath.Join(tmpDir, "POST-strip.xml")
	assert.FileExists(t, expectedFile)

	content, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "<update ")
	assert.Regexp(t, `<updates.*></updates>|</updates>`, string(content))
}

// TestSaveBody_NonXML verifies that non-XML bodies are saved without .xml extension.
func TestSaveBody_NonXML(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	body := []byte("plain text")
	req, _ := http.NewRequest("GET", "/plain", bytes.NewBuffer(body))

	hvac.SaveBody(req, body, true)

	expectedFile := filepath.Join(tmpDir, "GET-plain")
	assert.FileExists(t, expectedFile)
}

// TestCreateFileName_QueryString verifies query string inclusion when RequestURI is empty.
func TestCreateFileName_QueryString(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	req, _ := http.NewRequest("GET", "/path?foo=bar", nil)
	req.RequestURI = "" // force fallback path building

	filename := hvac.CreateFilePath(req, "", ".xml")
	assert.Contains(t, filename, "GET-path_foo=bar.xml")
}
