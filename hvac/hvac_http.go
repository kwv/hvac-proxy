package hvac

import (
	"bytes"
	"fmt"

	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

/**
This file contains functions to:
1. Save HTTP request/response bodies to disk
2. Decode URL-encoded HVAC form data
3. Update metrics from HVAC status XML
4. Generate safe, standardized file paths for saved content
**/

// SaveBody saves the HTTP request/response body to disk.
// Parameters:
// - r: the HTTP request
// - content: the raw byte content to save
// - isRequest: whether this is a request (vs response) body
func SaveBody(r *http.Request, content []byte, isRequest bool) {
	if len(content) == 0 {
		return
	}

	// Decode URL-encoded HVAC form data (e.g., "data=encoded%20value")
	if bytes.HasPrefix(content, []byte("data=")) {
		encoded := bytes.TrimPrefix(content, []byte("data="))
		if decoded, err := url.QueryUnescape(string(encoded)); err == nil {
			content = []byte(decoded)
		}
	}

	// If this is a request to the "/status" endpoint, update metrics from the XML content
	if strings.HasSuffix(r.URL.Path, "/status") && isRequest {
		SaveMetricsFromXML(content)
	}

	// Determine file extension based on content type
	var ext string
	if IsXML(content) {
		ext = ".xml"
	} else {
		ext = ""
	}

	// If BLOCK_UPDATES is enabled, remove any <update> blocks from the content
	blockUpdates := os.Getenv("BLOCK_UPDATES") == "true"
	if blockUpdates {
		re := regexp.MustCompile(`(?s)<update[^>]*>.*?</update>`)
		content = re.ReplaceAll(content, []byte{})
	}

	// Format XML content for readability (no-op for non-XML content)
	content = PrettifyXML(content)

	// Determine suffix based on whether this is a request or response
	var suffix string
	if !isRequest {
		suffix = "response"
	}

	// Generate a safe, standardized file path for the saved content
	filepath := CreateFilePath(r, suffix, ext)
	fmt.Printf("Saving body to %s\n", filepath)

	// Write the content to disk
	if err := os.WriteFile(filepath, content, 0644); err != nil {
		fmt.Printf("Failed to write file: %v\n", err)
	}
}

// CreateFilePath generates a safe, standardized file path for HTTP content.
// Parameters:
// - r: the HTTP request
// - suffix: "response" if this is a response, empty otherwise
// - extension: file extension (e.g., ".xml")
func CreateFilePath(r *http.Request, suffix string, extension string) string {
	// Use the request URI as the base path, falling back to URL.Path if needed
	path := r.RequestURI
	if path == "" {
		path = r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
	}
	path = strings.TrimPrefix(path, "/")

	// Construct the filename using HTTP method, path, suffix, and extension
	filename := r.Method + "-" + path
	if suffix != "" {
		filename += "-" + suffix
	}
	filename += extension

	// Clean and sanitize the filename to prevent invalid characters
	filename = filepath.Clean(filename)
	re := regexp.MustCompile(`[<>:"/\\|?*]+`)
	sanitized := re.ReplaceAllString(filename, "_")

	// Trim whitespace and limit filename length to 255 characters
	sanitized = strings.TrimSpace(sanitized)
	if len(sanitized) > 255 {
		sanitized = sanitized[:255]
	}

	// Write the file to the data directory
	filePath := filepath.Join(os.Getenv("DATA_DIR"), sanitized)
	return filePath
}
