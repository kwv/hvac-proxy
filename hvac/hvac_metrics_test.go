package hvac_test

import (
	"hvac-proxy/hvac"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPrometheus(t *testing.T) {
	status := hvac.Status{
		OAT:      63.5,
		IDU:      hvac.IDU{CFM: 437, OPSTAT: "off"},
		FiltrLvl: 40,
		Zones: hvac.Zones{
			Zones: []hvac.Zone{
				{CurrentTemp: 72.3, RelativeHumidity: 45, HeatSetPoint: 68.0, CoolSetPoint: 75.0},
			},
		},
		LocalTime: "2024-04-05T14:30:00Z",
	}
	actual := status.ToPrometheus()

	expected := `# HELP outdoorAirTemp degrees in F
# TYPE outdoorAirTemp gauge
outdoorAirTemp 63.5
# HELP fanSpeed cubic feet minute
# TYPE fanSpeed gauge
fanSpeed 437
# HELP Stage StageName
# TYPE Stage gauge
stage off
# HELP filter percent of filter life
# TYPE filter gauge
filter 40
# HELP temperature indoor temp
# TYPE temperature gauge
temperature 72.3
# HELP relativeHumidity indoor relative humidity
# TYPE relativeHumidity gauge
relativeHumidity 45
# HELP heatSetPoint heat set point
# TYPE heatSetPoint gauge
heatSetPoint 68.0
# HELP coolingSetPoint cooling set point
# TYPE coolingSetPoint gauge
coolingSetPoint 75.0
# HELP localtime last refreshed time
# TYPE localtime gauge
localtime 20240405143000
`

	assert.Equal(t, expected, actual)
}
