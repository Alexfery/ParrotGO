package timer

import (
	"strings"

	"parrot/internal/components"
)

// templateDir is the folder of the timer templates in templates/components.
const templateDir = "timer"

// templateData is what the timer templates see.
type templateData struct {
	Name         string // C symbol prefix, e.g. "heartbeat"
	Macro        string // C macro prefix, e.g. "HEARTBEAT"
	Period       string // as --period takes it, e.g. "1s"
	ResolutionHz int
	AlarmCount   uint64 // ticks per period
}

// Create generates the component under projectDir/components/<name> and
// returns the paths it created.
func Create(projectDir string, t Timer) ([]string, error) {
	data := templateData{
		Name:         t.Name,
		Macro:        strings.ToUpper(t.Name),
		Period:       FormatPeriod(t.PeriodUS),
		ResolutionHz: ResolutionHz,
		AlarmCount:   t.AlarmCount(),
	}
	return components.Generate(projectDir, templateDir, t.Name, data)
}
