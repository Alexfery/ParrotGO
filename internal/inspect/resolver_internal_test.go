package inspect

import (
	"os/exec"
	"strings"
	"testing"

	"parrot/internal/project"
	"parrot/internal/targets"
)

// No kind Parrot knows can form a loop of dependencies, so this test adds
// one: components of a loop must still be listed, with an error.
func TestResolveDependencyLoop(t *testing.T) {
	kinds["loop"] = kind{label: "Loop", group: "Loops", describe: func(e *entry, _ *targets.Target) {
		var cfg struct {
			On string `json:"on"`
		}
		if e.config.Decode(&cfg) == nil {
			e.dependsOn(requirement{role: "loop", name: cfg.On, typ: "loop", what: "a loop"})
		}
	}}
	t.Cleanup(func() { delete(kinds, "loop") })

	in := Resolve(project.Config{Target: "esp32", Components: []project.ComponentConfig{
		{Type: "loop", Name: "a", Config: []byte(`{"on": "b"}`)},
		{Type: "loop", Name: "b", Config: []byte(`{"on": "a"}`)},
		{Type: "loop", Name: "self", Config: []byte(`{"on": "self"}`)},
	}})
	// a is nested under b; b would close the loop, so it stays at the top level.
	if len(in.Components) != 2 || in.Components[0].Name != "b" || in.Components[1].Name != "self" {
		t.Fatalf("top level = %+v, want b and self", in.Components)
	}
	b := in.Components[0]
	if len(b.Children) != 1 || b.Children[0].Components[0].Name != "a" {
		t.Errorf("b children = %+v, want a", b.Children)
	}
	if want := `dependency "a" is built on "b" itself`; len(b.Issues) != 1 || b.Issues[0].Message != want {
		t.Errorf("b issues = %+v, want %q", b.Issues, want)
	}
	if want := `dependency "self" is built on "self" itself`; in.Components[1].Issues[0].Message != want {
		t.Errorf("self issues = %+v, want %q", in.Components[1].Issues, want)
	}
}

// Inspecting needs the project model, the target registry and the
// allocator, never ESP-IDF: the package must not even import the code that
// runs idf.py.
func TestNoESPIDFDependency(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool not found")
	}
	out, err := exec.Command(goTool, "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	deps := strings.Fields(string(out))
	if len(deps) == 0 {
		t.Fatal("go list printed no dependencies")
	}
	for _, dep := range deps {
		if dep == "parrot/internal/espidf" || dep == "parrot/internal/doctor" || dep == "os/exec" {
			t.Errorf("parrot/internal/inspect depends on %s", dep)
		}
	}
}
