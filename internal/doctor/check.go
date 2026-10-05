package doctor

// Status is the outcome of a check.
type Status int

const (
	StatusOK      Status = iota
	StatusWarning        // worth knowing, but Parrot can work
	StatusError          // Parrot cannot work until it is fixed
	StatusSkipped        // not run: there is nothing to check, or a check it needs did not pass
)

// CheckResult is the outcome of one check.
type CheckResult struct {
	Name    string // what was checked, e.g. "CMake"
	Status  Status
	Message string // details such as a version, or what is wrong; may span several lines
}

// Section groups the results of related checks.
type Section struct {
	Name    string
	Results []CheckResult
}

// Report is the outcome of a diagnosis, in the order the checks ran.
type Report struct {
	Sections []Section
}

// Count returns how many checks ended with status.
func (r Report) Count(status Status) int {
	n := 0
	for _, section := range r.Sections {
		for _, result := range section.Results {
			if result.Status == status {
				n++
			}
		}
	}
	return n
}

func ok(name, message string) CheckResult {
	return CheckResult{Name: name, Status: StatusOK, Message: message}
}

func warning(name, message string) CheckResult {
	return CheckResult{Name: name, Status: StatusWarning, Message: message}
}

func failed(name, message string) CheckResult {
	return CheckResult{Name: name, Status: StatusError, Message: message}
}

// skipped is the result of a check that could not run. Its cause is reported
// by its own check, so it is not counted as another problem.
func skipped(name, reason string) CheckResult {
	return CheckResult{Name: name, Status: StatusSkipped, Message: reason}
}
