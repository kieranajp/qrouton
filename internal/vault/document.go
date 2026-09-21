package vault

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Repository struct {
	Path     string  `yaml:"path" json:"path"`
	Role     string  `yaml:"role" json:"role"`
	Revision *string `yaml:"revision" json:"revision"`
}
type Edge struct {
	Relation string `yaml:"relation" json:"relation"`
	Target   string `yaml:"target" json:"target"`
}
type Document struct {
	SchemaVersion int          `yaml:"schema_version" json:"schemaVersion"`
	ID            string       `yaml:"id" json:"id"`
	Session       string       `yaml:"session" json:"session"`
	Kind          string       `yaml:"kind" json:"kind"`
	Title         string       `yaml:"title" json:"title"`
	Date          string       `yaml:"date" json:"date"`
	Author        string       `yaml:"author" json:"author"`
	State         string       `yaml:"state" json:"state"`
	StatusNote    string       `yaml:"status_note,omitempty" json:"statusNote,omitempty"`
	Repos         []Repository `yaml:"repos" json:"repos"`
	Workstream    string       `yaml:"workstream" json:"workstream"`
	Ticket        string       `yaml:"ticket,omitempty" json:"ticket,omitempty"`
	Lineage       []Edge       `yaml:"lineage,omitempty" json:"lineage,omitempty"`
	Body          string       `yaml:"-" json:"-"`
}

var componentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var revisionPattern = regexp.MustCompile(`^(?:[a-fA-F0-9]{40}|[a-fA-F0-9]{64})$`)
var repositoryPattern = regexp.MustCompile(`^github\.com/[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)

func ValidRepository(path string) bool { return repositoryPattern.MatchString(path) }
func validID(id string) bool {
	parts := strings.Split(id, "/")
	return len(parts) == 2 && componentPattern.MatchString(parts[0]) && componentPattern.MatchString(parts[1])
}

func Parse(data []byte) (Document, error) {
	var d Document
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return d, ErrInvalidDocument
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return d, ErrInvalidDocument
	}
	front := data[4 : 4+end]
	decoder := yaml.NewDecoder(bytes.NewReader(front))
	var node yaml.Node
	if err := decoder.Decode(&node); err != nil {
		return d, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return d, ErrInvalidDocument
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return d, ErrInvalidDocument
	}
	root := node.Content[0]
	if err := uniqueKeys(root); err != nil {
		return d, err
	}
	fields := nodeFields(root)
	version := fields["schema_version"]
	if version == nil || version.Tag != "!!int" || version.Decode(&d.SchemaVersion) != nil {
		return d, ErrInvalidDocument
	}
	d.Body = string(data[4+end+5:])
	if d.SchemaVersion != SchemaVersion {
		if id := fields["id"]; id != nil && id.Tag == "!!str" && validID(id.Value) {
			d.ID = id.Value
		}
		return d, ErrUnsupportedSchema
	}
	shape := map[string]string{"schema_version": "!!int", "id": "!!str", "session": "!!str", "kind": "!!str", "title": "!!str", "date": "date", "author": "!!str", "state": "!!str", "status_note": "!!str", "repos": "seq", "workstream": "!!str", "ticket": "!!str", "lineage": "seq"}
	if err := checkShape(root, shape); err != nil {
		return d, err
	}
	for _, key := range []string{"id", "session", "kind", "title", "date", "author", "state", "repos", "workstream"} {
		if fields[key] == nil {
			return d, fmt.Errorf("%w: missing %s", ErrInvalidDocument, key)
		}
	}
	for _, repo := range fields["repos"].Content {
		if err := checkShape(repo, map[string]string{"path": "!!str", "role": "!!str", "revision": "revision"}); err != nil {
			return d, err
		}
		if len(repo.Content) != 6 {
			return d, ErrInvalidDocument
		}
	}
	if lineage := fields["lineage"]; lineage != nil {
		for _, edge := range lineage.Content {
			if err := checkShape(edge, map[string]string{"relation": "!!str", "target": "!!str"}); err != nil {
				return d, err
			}
			if len(edge.Content) != 4 {
				return d, ErrInvalidDocument
			}
		}
	}
	body := d.Body
	if err := root.Decode(&d); err != nil {
		return d, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	d.Body = body
	return d, Validate(d)
}

func uniqueKeys(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return ErrInvalidDocument
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag != "!!str" || seen[key.Value] {
				return ErrInvalidDocument
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := uniqueKeys(child); err != nil {
			return err
		}
	}
	return nil
}
func nodeFields(node *yaml.Node) map[string]*yaml.Node {
	fields := map[string]*yaml.Node{}
	for i := 0; i < len(node.Content); i += 2 {
		fields[node.Content[i].Value] = node.Content[i+1]
	}
	return fields
}
func checkShape(node *yaml.Node, shape map[string]string) error {
	if node.Kind != yaml.MappingNode {
		return ErrInvalidDocument
	}
	for key, value := range nodeFields(node) {
		expected, ok := shape[key]
		if !ok {
			return fmt.Errorf("%w: unknown field %s", ErrInvalidDocument, key)
		}
		valid := value.Kind == yaml.ScalarNode && value.Tag == expected
		switch expected {
		case "seq":
			valid = value.Kind == yaml.SequenceNode
		case "date":
			valid = value.Kind == yaml.ScalarNode && (value.Tag == "!!str" || value.Tag == "!!timestamp")
		case "revision":
			valid = value.Kind == yaml.ScalarNode && (value.Tag == "!!str" || value.Tag == "!!null")
		}
		if !valid {
			return fmt.Errorf("%w: field %s", ErrInvalidDocument, key)
		}
	}
	return nil
}
func Validate(d Document) error {
	if d.SchemaVersion != SchemaVersion {
		return ErrUnsupportedSchema
	}
	if !validID(d.ID) || !componentPattern.MatchString(d.Session) || !strings.HasPrefix(d.ID, d.Session+"/") {
		return ErrInvalidDocument
	}
	for _, value := range []string{d.Title, d.Author, d.Workstream} {
		if strings.TrimSpace(value) == "" {
			return ErrInvalidDocument
		}
	}
	if _, err := time.Parse("2006-01-02", d.Date); err != nil {
		return ErrInvalidDocument
	}
	switch d.Kind {
	case "research", "spec", "plan", "note", "dead_end":
	default:
		return ErrInvalidDocument
	}
	switch d.State {
	case "active", "superseded", "abandoned":
	default:
		return ErrInvalidDocument
	}
	if d.Kind != "note" && len(d.Repos) == 0 {
		return ErrInvalidDocument
	}
	for _, repo := range d.Repos {
		if !ValidRepository(repo.Path) || (repo.Role != "editing" && repo.Role != "reference") {
			return ErrInvalidDocument
		}
		if repo.Revision == nil {
			if d.Kind != "note" {
				return ErrInvalidDocument
			}
		} else if !revisionPattern.MatchString(*repo.Revision) {
			return ErrInvalidDocument
		}
	}
	if d.Ticket != "" {
		u, err := url.Parse(d.Ticket)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
			return ErrInvalidDocument
		}
	}
	for _, edge := range d.Lineage {
		if (edge.Relation != "derived_from" && edge.Relation != "supersedes") || !validID(edge.Target) || edge.Target == d.ID {
			return ErrInvalidDocument
		}
	}
	return nil
}
func Encode(d Document) ([]byte, error) {
	if err := Validate(d); err != nil {
		return nil, err
	}
	if d.Repos == nil {
		d.Repos = []Repository{}
	}
	b, err := yaml.Marshal(d)
	if err != nil {
		return nil, err
	}
	return []byte("---\n" + string(b) + "---\n" + d.Body), nil
}
