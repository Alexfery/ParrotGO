package doctor

import (
	"errors"
	"fmt"
	"strings"

	"parrot/internal/espidf"
	"parrot/internal/project"
	"parrot/internal/targets"
)

// project checks the Parrot project in d.ProjectDir, if there is one. Not
// being in a project is not a problem: the environment checks are still
// useful. parrot.json and CMakeLists.txt are checked independently; the
// target needs a readable parrot.json.
func (d Doctor) project() []CheckResult {
	cfg, err := project.LoadConfigFrom(d.ProjectDir)
	if errors.Is(err, project.ErrNotProject) {
		return []CheckResult{skipped("Parrot project", "No Parrot project detected (no parrot.json in this directory).")}
	}

	results := []CheckResult{checkConfig(err), checkCMakeLists(d.ProjectDir)}
	if err != nil {
		return append(results, skipped("Target", "Skipped: requires a valid parrot.json."))
	}
	return append(results, checkTarget(cfg.Target))
}

func checkConfig(loadErr error) CheckResult {
	if loadErr != nil {
		return failed(project.ConfigFile, "Invalid Parrot configuration.\n"+loadErr.Error())
	}
	return ok(project.ConfigFile, "")
}

func checkCMakeLists(dir string) CheckResult {
	if err := espidf.CheckProject(dir); err != nil {
		return failed("CMakeLists.txt", err.Error())
	}
	return ok("CMakeLists.txt", "")
}

// checkTarget checks the target of parrot.json against the target registry.
func checkTarget(id string) CheckResult {
	target, err := targets.Get(id)
	if err != nil {
		var ids []string
		for _, t := range targets.All() {
			ids = append(ids, t.ID)
		}
		return failed("Target", fmt.Sprintf("Unsupported target %q.\nSupported targets: %s.", id, strings.Join(ids, ", ")))
	}
	return ok("Target", fmt.Sprintf("%s (ESP-IDF target %s)", target.DisplayName, target.IDFTarget))
}
