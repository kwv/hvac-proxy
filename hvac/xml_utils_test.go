package hvac_test

import (
	"testing"

	"hvac-proxy/hvac"

	"github.com/stretchr/testify/assert"
)

// TestPrettifyXML_ValidXML verifies that valid XML is prettified.
func TestPrettifyXML_ValidXML(t *testing.T) {
	input := []byte(`<status><oat>63</oat></status>`)
	expected := `<status>
  <oat>63</oat>
</status>`

	result := string(hvac.PrettifyXML(input))
	assert.Equal(t, expected, result)
}

// TestPrettifyXML_InvalidXML verifies that invalid XML is returned as is.
func TestPrettifyXML_InvalidXML(t *testing.T) {
	input := []byte(`<invalid><data>test</invalid>`)
	result := hvac.PrettifyXML(input)
	assert.Equal(t, input, result)
}

// TestPrettifyXML_NonXML verifies that non-XML content is returned as is.
func TestPrettifyXML_NonXML(t *testing.T) {
	input := []byte("This is not XML content")
	result := hvac.PrettifyXML(input)
	assert.Equal(t, input, result)
}

// TestIsXML_ValidXML verifies that valid XML returns true.
func TestIsXML_ValidXML(t *testing.T) {
	input := []byte(`<status><oat>63</oat></status>`)
	assert.True(t, hvac.IsXML(input))
}

// TestIsXML_InvalidXML verifies that invalid XML returns false.
func TestIsXML_InvalidXML(t *testing.T) {
	input := []byte(`<invalid><data>test</invalid>`)
	assert.False(t, hvac.IsXML(input))
}

// TestIsXML_EmptyInput verifies that empty input returns false.
func TestIsXML_EmptyInput(t *testing.T) {
	assert.False(t, hvac.IsXML([]byte("")))
}

// TestIsXML_IncompleteXML verifies that incomplete XML returns false.
func TestIsXML_IncompleteXML(t *testing.T) {
	input := []byte(`<status><oat>63`)
	assert.False(t, hvac.IsXML(input))
}

// TestIsXML_IncompleteXML verifies that incomplete XML returns false.
func TestIsXML_WITHNAMESPACESXML(t *testing.T) {
	input := []byte(`<updates xmlns="http://schema.ota.carrier.com" xmlns="http://schema.ota.carrier.com"><update xmlns="http://schema.ota.carrier.com"><type xmlns="http://schema.ota.carrier.com">thermostat</type><model xmlns="http://schema.ota.carrier.com">SYSTXCCITC01-A</model><locales xmlns="http://schema.ota.carrier.com"><locale xmlns="http://schema.ota.carrier.com">en-us</locale></locales><version xmlns="http://schema.ota.carrier.com">14.02</version><url xmlns="http://schema.ota.carrier.com">http://www.ota.ing.carrier.com/updates/systxccit-14.02.hex</url><releaseNotes xmlns="http://schema.ota.carrier.com"><url xmlns="http://schema.ota.carrier.com" type="text/plain" locale="en-us">http://www.ota.ing.carrier.com/releaseNotes/systxccit-14.02.txt</url><url xmlns="http://schema.ota.carrier.com" type="text/html" locale="en-us">http://www.ota.ing.carrier.com/releaseNotes/systxccit-14.02.html</url></releaseNotes></update></updates>`)
	assert.True(t, hvac.IsXML(input))
}
