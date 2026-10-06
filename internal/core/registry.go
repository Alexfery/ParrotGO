package core

import (
	"fmt"
	"slices"
	"strings"

	"parrot/internal/project"
)

// Registry holds the platforms Parrot supports, by ID. Supporting a new
// platform means registering it, not changing the commands. The zero value
// is an empty registry ready to use.
type Registry struct {
	platforms map[string]Platform
}

// NewRegistry returns a registry holding platforms.
func NewRegistry(platforms ...Platform) *Registry {
	r := &Registry{}
	for _, p := range platforms {
		r.Register(p)
	}
	return r
}

// Register adds p to the registry. It panics if p has an empty ID or one
// already registered: like http.ServeMux.Handle, registering is part of
// building the program, not something that can fail at run time.
func (r *Registry) Register(p Platform) {
	id := p.ID()
	if id == "" {
		panic("core: Register of a platform with an empty ID")
	}
	if _, taken := r.platforms[id]; taken {
		panic(fmt.Sprintf("core: platform %q registered twice", id))
	}
	if r.platforms == nil {
		r.platforms = make(map[string]Platform)
	}
	r.platforms[id] = p
}

// Get returns the platform with the given ID, or an error listing the
// supported platforms if there is none.
func (r *Registry) Get(id string) (Platform, error) {
	if p, found := r.platforms[id]; found {
		return p, nil
	}
	return nil, fmt.Errorf("unsupported platform %q\n\nSupported platforms:\n%s", id, r.list())
}

// OpenProject loads the parrot.json in dir and has the platform it names
// open the project (see Platform.OpenProject). A parrot.json without a
// platform is an ESP32 project, as every project was before Parrot had
// platforms (see project.Config.PlatformID).
func (r *Registry) OpenProject(dir string, stdio IO) (Project, error) {
	cfg, err := project.LoadConfigFrom(dir)
	if err != nil {
		return nil, err
	}
	platform, err := r.Get(cfg.PlatformID())
	if err != nil {
		return nil, err
	}
	return platform.OpenProject(dir, cfg, stdio)
}

// list returns the registered IDs, sorted, one per indented line.
func (r *Registry) list() string {
	ids := make([]string, 0, len(r.platforms))
	for id := range r.platforms {
		ids = append(ids, "  "+id)
	}
	slices.Sort(ids)
	return strings.Join(ids, "\n")
}
