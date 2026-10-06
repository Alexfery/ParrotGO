package inspect

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"parrot/internal/components"
	"parrot/internal/components/catalog"
	"parrot/internal/project"
	"parrot/internal/resources"
	"parrot/internal/targets"
)

// Resolve analyses cfg. It never fails and never panics on a broken
// manifest: every problem it finds is an Issue of the result, so that the
// project can still be explained.
//
// parrot.json lists components flat; Resolve turns the list into trees, in
// four passes over it:
//
//  1. index: map every name to its component, and flag duplicate names and
//     unknown types;
//  2. allocate: run the resource allocator on the components' claims, as
//     code generation does;
//  3. describe: let each known type decode its config, derive what Parrot
//     computes and declare the component it is built on;
//  4. resolve: find each declared dependency through the index and nest the
//     component under it.
func Resolve(cfg project.Config) Inspection {
	r := newResolver(cfg)
	r.allocate()
	r.describe()
	r.resolveDependencies()
	return Inspection{Target: r.targetInfo(), Components: r.tree()}
}

// resolver holds the state of one Resolve call.
type resolver struct {
	cfg          project.Config
	target       *targets.Target // nil if the target is not supported
	targetIssues []Issue

	entries []*entry          // every component, in manifest order
	byName  map[string]*entry // the component index: the first component of each name
}

// entry is a component of parrot.json while it is resolved.
type entry struct {
	config    project.ComponentConfig
	kind      *kind     // nil if Parrot does not know the type
	component Component // the result, without its Children

	duplicate   bool         // an earlier component has the same name
	requirement *requirement // the component it is built on, if any
	parent      *entry       // set once requirement is resolved
	children    []*entry     // the components built on it, in manifest order

	// allocation is the project's resource allocation, as code generation
	// computes it, if it covers this component; otherwise blockedBy names the
	// component it stopped at.
	allocation *resources.Allocation
	blockedBy  string
}

// requirement is a dependency as a kind declares it, before it is resolved.
type requirement struct {
	role string // what the dependency is to the component, e.g. "bus"
	name string // the component named in the config
	typ  string // the type that component must have
	what string // how errors name that type, e.g. "an I2C bus"

	// check applies the rules the component has for its dependency once the
	// dependency is found with the right type, e.g. the addresses a BME280
	// accepts; nil if there are none.
	check func(dependency project.ComponentConfig) error
}

// newResolver indexes the components of cfg by name, in one pass, so that
// resolving a dependency is a map lookup instead of a search.
func newResolver(cfg project.Config) *resolver {
	r := &resolver{cfg: cfg, byName: make(map[string]*entry, len(cfg.Components))}
	if t, err := targets.Get(cfg.Target); err == nil {
		r.target = &t
	} else {
		r.targetIssues = []Issue{{SeverityError, unsupportedTarget(cfg.Target)}}
	}

	for _, c := range cfg.Components {
		e := &entry{config: c, component: Component{Name: c.Name, Type: c.Type}}
		if _, err := components.NormalizeName(c.Name); err != nil {
			e.fail(err)
		}
		if _, taken := r.byName[c.Name]; taken {
			e.duplicate = true
			e.fail(fmt.Errorf("duplicate component name %q: an earlier component in %s has it", c.Name, project.ConfigFile))
		} else {
			r.byName[c.Name] = e
		}
		if k, known := kinds[c.Type]; known {
			e.kind = &k
			e.component.Label = k.label
		} else {
			e.warn("unknown component type: this version of Parrot cannot inspect it")
		}
		r.entries = append(r.entries, e)
	}
	return r
}

// unsupportedTarget is the issue of a target missing from the registry.
func unsupportedTarget(id string) string {
	var ids []string
	for _, t := range targets.All() {
		ids = append(ids, t.ID)
	}
	return fmt.Sprintf("unsupported target %q (supported: %s)", id, strings.Join(ids, ", "))
}

// allocate runs the resource allocator on the claims of the components, in
// manifest order, exactly as code generation does: it uses the same claims
// (internal/components/catalog) and the same allocator (internal/resources),
// so both reach the same LEDC timers and channels and SPI hosts. It also
// reports resource conflicts on the component that causes them.
//
// The allocation of a component only depends on the components before it,
// so it is known for every component before the first one Parrot cannot
// account for: an unknown type, a duplicate name, an invalid config or a
// conflict. Generation would refuse that component, and the allocation of
// the ones after it cannot be known.
func (r *resolver) allocate() {
	if r.target == nil {
		return // nothing can be allocated; the target issue says why
	}
	stop := len(r.entries)
	claims := make([]resources.Claim, 0, len(r.entries))
	for i, e := range r.entries {
		if e.kind == nil || e.duplicate {
			stop = i
			break
		}
		claim, err := catalog.Claim(e.config, *r.target)
		if err != nil {
			e.fail(err)
			stop = i
			break
		}
		claims = append(claims, claim)
	}

	alloc, err := resources.Allocate(*r.target, claims)
	var conflict *resources.ClaimError
	if errors.As(err, &conflict) {
		// claims[i] is the claim of r.entries[i], and the names are unique:
		// duplicates stopped the claims above.
		stop = slices.IndexFunc(claims, func(c resources.Claim) bool { return c.Component == conflict.Component })
		r.entries[stop].fail(conflict.Err)
		// The claims before the conflict were satisfied, so they still are.
		alloc, _ = resources.Allocate(*r.target, claims[:stop])
	}

	for i, e := range r.entries {
		switch {
		case i < stop:
			e.allocation = &alloc
		case i > stop:
			e.blockedBy = r.entries[stop].config.Name
		}
	}
}

// describe lets the kind of each component describe it.
func (r *resolver) describe() {
	for _, e := range r.entries {
		if e.kind != nil {
			e.kind.describe(e, r.target)
		}
	}
}

// resolveDependencies finds, through the index, the component each one is
// built on, and records it as the component's parent when it exists with
// the right type. A component whose dependency cannot be resolved stays at
// the top level, with an issue.
func (r *resolver) resolveDependencies() {
	for _, e := range r.entries {
		req := e.requirement
		if req == nil {
			continue
		}
		dep := Dependency{Role: req.role, Name: req.name, Type: req.typ}
		parent, found := r.byName[req.name]
		switch {
		case !found:
			e.fail(fmt.Errorf("dependency %q not found", req.name))
		case parent.config.Type != req.typ:
			e.fail(fmt.Errorf("%q is not %s (its type is %q)", req.name, req.what, parent.config.Type))
		case parent.builtOn(e):
			e.fail(fmt.Errorf("dependency %q is built on %q itself", req.name, e.config.Name))
		default:
			dep.Resolved = true
			e.parent = parent
			if req.check != nil {
				if err := req.check(parent.config); err != nil {
					e.fail(err)
				}
			}
		}
		e.component.Dependencies = append(e.component.Dependencies, dep)
	}
}

// builtOn reports whether e is other or is built on it, directly or not:
// nesting other under e would then make a loop. The types Parrot knows
// cannot form one (a sensor is built on a device, a device on a bus, a bus
// on nothing), but the components of a loop would never reach the top level
// and would vanish from the trees.
func (e *entry) builtOn(other *entry) bool {
	for a := e; a != nil; a = a.parent {
		if a == other {
			return true
		}
	}
	return false
}

// tree nests every component under its parent and returns the top-level
// ones, all in manifest order.
func (r *resolver) tree() []Component {
	var roots []*entry
	for _, e := range r.entries {
		if e.parent == nil {
			roots = append(roots, e)
		} else {
			e.parent.children = append(e.parent.children, e)
		}
	}
	out := make([]Component, 0, len(roots))
	for _, e := range roots {
		out = append(out, e.build())
	}
	return out
}

// build returns e's component with its children, grouped by the group of
// their kind, in the order the groups first appear in the manifest.
func (e *entry) build() Component {
	c := e.component
	for _, child := range e.children {
		name := child.kind.group // a child has a kind: only kinds declare dependencies
		i := slices.IndexFunc(c.Children, func(g Group) bool { return g.Name == name })
		if i < 0 {
			c.Children = append(c.Children, Group{Name: name})
			i = len(c.Children) - 1
		}
		c.Children[i].Components = append(c.Children[i].Components, child.build())
	}
	return c
}

func (r *resolver) targetInfo() TargetInfo {
	info := TargetInfo{ID: r.cfg.Target, Issues: r.targetIssues}
	if r.target != nil {
		info.DisplayName = r.target.DisplayName
		info.IDFTarget = r.target.IDFTarget
	}
	return info
}

// add appends properties to the component, in display order.
func (e *entry) add(properties ...Property) {
	e.component.Properties = append(e.component.Properties, properties...)
}

// dependsOn declares the component e is built on.
func (e *entry) dependsOn(req requirement) {
	e.requirement = &req
}

// check records err, if any, as an error of the component.
func (e *entry) check(err error) {
	if err != nil {
		e.fail(err)
	}
}

func (e *entry) fail(err error) {
	e.report(SeverityError, err.Error())
}

func (e *entry) warn(message string) {
	e.report(SeverityWarning, message)
}

// unallocated warns that what the allocator gives the component cannot be
// known, if the allocation stopped before it. A component the allocation
// stopped at has an error saying why.
func (e *entry) unallocated(what string) {
	if e.blockedBy != "" {
		e.warn(fmt.Sprintf("%s cannot be determined: %q comes earlier in %s and cannot be allocated",
			what, e.blockedBy, project.ConfigFile))
	}
}

// report records an issue once: some problems are found twice, e.g. an
// invalid config by both the claim and the kind.
func (e *entry) report(severity Severity, message string) {
	for _, issue := range e.component.Issues {
		if issue.Message == message {
			return
		}
	}
	e.component.Issues = append(e.component.Issues, Issue{severity, message})
}
