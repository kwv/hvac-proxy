package hvac

import (
	"bytes"
	"encoding/xml"
	"io"
)

// PrettifyXML formats raw XML with indentation.
func PrettifyXML(input []byte) []byte {
	// Check if it's XML content
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) == 0 || (trimmed[0] != '<' && !bytes.HasPrefix(trimmed, []byte("<?xml"))) {
		return input
	}

	var buf bytes.Buffer
	decoder := xml.NewDecoder(bytes.NewReader(input))
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return input // Return original on error
		}
		if err := encoder.EncodeToken(token); err != nil {
			return input // Return original on error
		}
	}
	encoder.Flush()

	if buf.Len() > 0 {
		return buf.Bytes()
	}
	return input
}

func IsXML(data []byte) bool {
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
