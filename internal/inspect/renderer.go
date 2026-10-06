package inspect

import (
	"fmt"
	"io"
	"strings"
)

// TextRenderer writes an Inspection as plain text: the target, then one tree
// per top-level component. It uses no colors, so the output reads the same
// in a terminal, in a file and in a CI log.
type TextRenderer struct {
	Writer io.Writer
}

// Render writes inspection to r.Writer. It only reads the Inspection, never
// parrot.json.
func (r *TextRenderer) Render(inspection Inspection) error {
	var b strings.Builder
	b.WriteString("Parrot Project\n\n")

	writeHeading(&b, "Target")
	writeTarget(&b, inspection.Target)
	b.WriteString("\n")

	writeHeading(&b, "Components")
	if len(inspection.Components) == 0 {
		b.WriteString("No components yet. Add one with parrot add.\n")
	}
	for i, c := range inspection.Components {
		if i > 0 {
			b.WriteString("\n")
		}
		writeTree(&b, topLevel(c))
	}

	_, err := io.WriteString(r.Writer, b.String())
	return err
}

func writeHeading(b *strings.Builder, title string) {
	fmt.Fprintf(b, "%s\n%s\n\n", title, strings.Repeat("-", len(title)))
}

func writeTarget(b *strings.Builder, t TargetInfo) {
	if t.DisplayName != "" {
		fmt.Fprintf(b, "%s\nParrot ID: %s\nESP-IDF target: %s\n", t.DisplayName, t.ID, t.IDFTarget)
	}
	for _, issue := range t.Issues {
		b.WriteString(issueText(issue) + "\n")
	}
}

// line is one line of a tree, with the lines nested under it.
type line struct {
	text     string
	children []line
}

// topLevel is a component at the top level: its name, with its type below.
//
//	status
//	└── LED
//	    └── GPIO4
func topLevel(c Component) line {
	return line{text: orQuotes(c.Name), children: []line{{text: label(c), children: details(c)}}}
}

// nested is a component listed under the one it is built on: its name and
// type on one line, as the group above already says what it is to its parent.
//
//	└── Devices
//	    └── display (I2C Device)
//	        └── Address: 0x3C
func nested(c Component) line {
	return line{text: fmt.Sprintf("%s (%s)", orQuotes(c.Name), label(c)), children: details(c)}
}

// details are the lines under a component: its properties, its issues, then
// the components built on it, by group.
func details(c Component) []line {
	lines := make([]line, 0, len(c.Properties)+len(c.Issues)+len(c.Children))
	for _, p := range c.Properties {
		lines = append(lines, line{text: propertyText(p)})
	}
	for _, issue := range c.Issues {
		lines = append(lines, line{text: issueText(issue)})
	}
	for _, g := range c.Children {
		group := line{text: g.Name}
		for _, child := range g.Components {
			group.children = append(group.children, nested(child))
		}
		lines = append(lines, group)
	}
	return lines
}

// label names the component's type: its human name, or the type as written
// in parrot.json when Parrot does not know it.
func label(c Component) string {
	if c.Label != "" {
		return c.Label
	}
	return orQuotes(c.Type)
}

func propertyText(p Property) string {
	if p.Name == "" {
		return p.Value
	}
	return p.Name + ": " + p.Value
}

func issueText(issue Issue) string {
	if issue.Severity == SeverityWarning {
		return "WARNING: " + issue.Message
	}
	return "ERROR: " + issue.Message
}

// orQuotes shows an empty name or type as "", rather than as nothing.
func orQuotes(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

// writeTree writes root at the left margin and the lines under it with
// box-drawing characters.
func writeTree(b *strings.Builder, root line) {
	b.WriteString(root.text + "\n")
	writeChildren(b, root.children, "")
}

// writeChildren writes lines below a parent whose own lines start with
// prefix. A line that is not the last one keeps a vertical bar in front of
// its children, so that the next sibling stays connected.
func writeChildren(b *strings.Builder, lines []line, prefix string) {
	for i, l := range lines {
		branch, indent := "├── ", "│   "
		if i == len(lines)-1 {
			branch, indent = "└── ", "    "
		}
		b.WriteString(prefix + branch + l.text + "\n")
		writeChildren(b, l.children, prefix+indent)
	}
}
