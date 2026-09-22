package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type ImportSession struct {
	Key                 string       `json:"key"`
	Origin              string       `json:"origin"`
	Content             []byte       `json:"content"`
	Slug                string       `json:"slug"`
	Workstream          string       `json:"workstream"`
	Ticket              string       `json:"ticket"`
	Repositories        []Repository `json:"repositories"`
	PublicationDisabled bool         `json:"publicationDisabled"`
}
type ImportSource struct {
	RelativePath string         `json:"relativePath,omitempty"`
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Origin       string         `json:"origin"`
	Content      []byte         `json:"content"`
	Session      *ImportSession `json:"session,omitempty"`
}
type ImportOverride struct {
	Document    Document `json:"document"`
	Body        *string  `json:"body,omitempty"`
	Destination string   `json:"destination,omitempty"`
}
type ImportRequest struct {
	prepared          map[string]ImportEntry
	targetProfile     string
	Sources           []ImportSource            `json:"sources"`
	Overrides         map[string]ImportOverride `json:"overrides,omitempty"`
	RepositoryAliases map[string]string         `json:"repositoryAliases,omitempty"`
	Namespaces        map[string]string         `json:"namespaces,omitempty"`
	LinkMappings      map[string]string         `json:"linkMappings,omitempty"`
}
type Repair struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
type ImportEntry struct {
	EvidenceKeys []string                 `json:"evidenceKeys,omitempty"`
	Disposition  string                   `json:"disposition,omitempty"`
	Reason       string                   `json:"reason,omitempty"`
	Provenance   map[string]FieldEvidence `json:"provenance,omitempty"`
	Path         string                   `json:"path"`
	Body         string                   `json:"body"`
	Dependencies []string                 `json:"dependencies,omitempty"`
	Key          string                   `json:"key"`
	Name         string                   `json:"name"`
	Document     Document                 `json:"document"`
	Canonical    string                   `json:"canonical"`
	Destination  string                   `json:"destination"`
	Repairs      []Repair                 `json:"repairs"`
	Warnings     []Repair                 `json:"warnings"`
	Unsupported  map[string]string        `json:"unsupported"`
}
type ImportPreview struct {
	ID             string        `json:"id"`
	Entries        []ImportEntry `json:"entries"`
	Accepted       int           `json:"accepted"`
	RepairRequired int           `json:"repairRequired"`
	Excluded       int           `json:"excluded"`
}
type importRecord struct {
	Sessions       map[string]ImportSession `json:"sessions,omitempty"`
	SourceSessions map[string]string        `json:"sourceSessions,omitempty"`
	Version        int                      `json:"version"`
	Preview        ImportPreview            `json:"preview"`
	Sources        []ImportSource           `json:"sources"`
	Hashes         map[string]string        `json:"hashes"`
	Roots          map[string]string        `json:"roots"`
}

var legacyFilename = regexp.MustCompile(`^([RSPND][0-9]+)(?:-([0-9]{4}-[0-9]{2}-[0-9]{2})(?:-.*)?)?$`)

func SourceSessionKey(manifestPath string) string {
	return contentHash(canonicalProfileRoot(manifestPath))
}
func (s *Service) PreviewImport(ctx context.Context, request ImportRequest) (ImportPreview, error) {
	directory, err := s.importDirectory()
	if err != nil {
		return ImportPreview{}, err
	}
	if err := ValidateProfiles(s.profiles, s.mappings); err != nil {
		return ImportPreview{}, err
	}
	if len(request.Sources) == 0 || len(request.Sources) > 4096 {
		return ImportPreview{}, ErrImportSource
	}
	sourceKeys := map[string]bool{}
	for _, source := range request.Sources {
		sourceKeys[source.Key] = true
	}
	for _, source := range request.Sources {
		if source.Session != nil && sourceKeys[source.Session.Key] {
			return ImportPreview{}, ErrImportSource
		}
	}
	record := importRecord{Version: 1, Sources: request.Sources, Hashes: map[string]string{}, Roots: map[string]string{}}
	record.Preview.ID, err = randomName("import-")
	if err != nil {
		return ImportPreview{}, err
	}
	seen := map[string]bool{}
	sourcesByKey := map[string]ImportSource{}
	total := 0
	for _, source := range request.Sources {
		if err := ctx.Err(); err != nil {
			return ImportPreview{}, err
		}
		total += len(source.Content)
		if source.Key == "" || seen[source.Key] || len(source.Content) > maxDocumentBytes || total > 64<<20 {
			return ImportPreview{}, ErrImportSource
		}
		seen[source.Key] = true
		sourcesByKey[source.Key] = source
		record.Hashes[source.Key] = contentHash(string(source.Content))
		if source.Session != nil {
			if _, seen := record.Hashes[source.Session.Key]; !seen {
				total += len(source.Session.Content)
			}
			if total > 64<<20 {
				return ImportPreview{}, ErrImportSource
			}
			if source.Session.Key == "" || len(source.Session.Content) == 0 || len(source.Session.Content) > maxDocumentBytes {
				return ImportPreview{}, ErrImportSource
			}
			hash := contentHash(string(source.Session.Content))
			if prior, exists := record.Hashes[source.Session.Key]; exists && prior != hash {
				return ImportPreview{}, ErrImportSource
			}
			record.Hashes[source.Session.Key] = hash
		}
		entry, prepared := request.prepared[source.Key]
		if !prepared {
			entry = normalizeSource(source, request)
		}
		scope := Scope{Destination: request.Overrides[source.Key].Destination}
		for _, repo := range entry.Document.Repos {
			scope.Repositories = append(scope.Repositories, repo.Path)
		}
		destination, routeErr := ResolveDestination(s.profiles, s.mappings, scope)
		if routeErr != nil {
			entry.Repairs = append(entry.Repairs, Repair{"destination", ErrDestinationRequired.Error()})
		} else {
			entry.Destination = destination
			for _, profile := range s.profiles {
				if profile.ID == destination {
					record.Roots[source.Key] = canonicalProfileRoot(profile.Root)
				}
				if source.Origin != "" && insideRoot(profile.Root, source.Origin) && profile.ID != destination {
					entry.Repairs = append(entry.Repairs, Repair{"destination", ErrCrossVaultImport.Error()})
				}
			}
		}
		if request.targetProfile != "" {
			s.classifyCorpusDestination(&entry, request.targetProfile)
		}
		record.Preview.Entries = append(record.Preview.Entries, entry)
	}
	inventories := map[string]inventory{}
	for _, profile := range s.profiles {
		if inv, err := scan(profile); err == nil {
			if owner := s.identityOwner(profile.ID); owner != nil {
				inv = owner.applyIdentityConflicts(inv)
			}
			inventories[profile.ID] = inv
		}
	}
	existingIDs := map[string]map[string][]ReadResult{}
	existingPaths := map[string]map[string]ReadResult{}
	for profile, inv := range inventories {
		existingIDs[profile] = map[string][]ReadResult{}
		existingPaths[profile] = map[string]ReadResult{}
		for _, entry := range inv.entries {
			existingIDs[profile][entry.Document.ID] = append(existingIDs[profile][entry.Document.ID], entry)
			existingPaths[profile][entry.Reference.Path] = entry
		}
	}
	for i := range record.Preview.Entries {
		entry := &record.Preview.Entries[i]
		entry.Path = entry.Document.ID + ".md"
		for _, existing := range existingIDs[entry.Destination][entry.Document.ID] {
			if existing.Supported && !inventories[entry.Destination].conflicts[existing.Document.ID] {
				entry.Path = existing.Reference.Path
				break
			}
		}
	}
	linkIndex := newImportLinkIndex(record.Preview.Entries, request.Sources)
	for index := range record.Preview.Entries {
		entry := &record.Preview.Entries[index]
		source := request.Sources[index]
		if entry.Disposition == "excluded" {
			entry.Body = entry.Document.Body
			continue
		}
		body, issues, dependencies := normalizeImportLinksIndexed(entry.Document.Body, source, *entry, request.LinkMappings, linkIndex)
		entry.Document.Body = body
		entry.Body = body
		entry.Dependencies = dependencies
		entry.Repairs = append(entry.Repairs, issues...)
		for edgeIndex, edge := range entry.Document.Lineage {
			target, ok := linkIndex.resolve(edge.Target, source, *entry)
			if mapped := request.Namespaces[strings.SplitN(edge.Target, "/", 2)[0]]; mapped != "" && validID(edge.Target) {
				entry.Document.Lineage[edgeIndex].Target = mapped + "/" + strings.SplitN(edge.Target, "/", 2)[1]
			}
			if ok {
				entry.Document.Lineage[edgeIndex].Target = target.Document.ID
				if target.Key != entry.Key {
					entry.Dependencies = append(entry.Dependencies, target.Key)
				}
			} else if !validID(edge.Target) {
				entry.Repairs = append(entry.Repairs, Repair{"lineage", ErrImportLink.Error()})
			}
		}
	}
	counts := map[string]int{}
	graphs := map[string]map[string]Document{}
	lineage := map[string]map[string]LineageState{}
	for profile, inv := range inventories {
		graphs[profile] = importGraph(inv)
	}
	for _, entry := range record.Preview.Entries {
		if entry.Disposition == "excluded" {
			continue
		}
		counts[entry.Destination+"\x00"+entry.Document.ID]++
		if graphs[entry.Destination] == nil {
			graphs[entry.Destination] = map[string]Document{}
		}
		graphs[entry.Destination][entry.Document.ID] = entry.Document
	}
	for profile, graph := range graphs {
		lineage[profile] = ResolveLineage(graph)
	}
	for index := range record.Preview.Entries {
		entry := &record.Preview.Entries[index]
		if entry.Disposition == "excluded" {
			continue
		}
		if counts[entry.Destination+"\x00"+entry.Document.ID] > 1 {
			entry.Repairs = append(entry.Repairs, Repair{"id", ErrConflict.Error()})
		}
		if lineage[entry.Destination][entry.Document.ID].Invalid {
			entry.Repairs = append(entry.Repairs, Repair{"lineage", ErrInvalidDocument.Error()})
		}
		entry.Repairs = append(entry.Repairs, metadataRepairs(entry.Document)...)
		canonical, encodeErr := Encode(entry.Document)
		source := sourcesByKey[entry.Key]
		if original, err := Parse(source.Content); err == nil {
			encoded, _ := Encode(original)
			if bytes.Equal(encoded, canonical) {
				canonical = source.Content
			}
		}
		if encodeErr != nil {
			entry.Repairs = append(entry.Repairs, Repair{"metadata", encodeErr.Error()})
		} else {
			entry.Canonical = string(canonical)
		}
		if entry.Destination != "" && encodeErr == nil {
			inv := inventories[entry.Destination]
			if inv.conflicts[entry.Document.ID] {
				entry.Repairs = append(entry.Repairs, Repair{"id", ErrConflict.Error()})
			}
			for _, existing := range existingIDs[entry.Destination][entry.Document.ID] {
				if existing.Content != entry.Canonical {
					entry.Repairs = append(entry.Repairs, Repair{"id", ErrConflict.Error()})
					break
				}
			}
			if existing, ok := existingPaths[entry.Destination][entry.Path]; ok && existing.Document.ID != entry.Document.ID {
				entry.Repairs = append(entry.Repairs, Repair{"id", ErrConflict.Error()})
			}
		}
		if strings.Contains(entry.Body, "repo://github.com/") {
			entry.Warnings = append(entry.Warnings, Repair{"body", corpusUnknownCitation})
		}
	}
	finalizeImportPreview(&record.Preview, request.targetProfile != "")
	sort.Slice(record.Preview.Entries, func(i, j int) bool { return record.Preview.Entries[i].Key < record.Preview.Entries[j].Key })
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ImportPreview{}, ErrImportState
	}
	defer root.Close()
	encoded, err := encodeImportRecord(record, maxImportRecordBytes)
	if err != nil {
		return ImportPreview{}, err
	}
	if err := replaceRoot(root, record.Preview.ID+".json", encoded); err != nil {
		return ImportPreview{}, err
	}
	return record.Preview, nil
}
func normalizeSource(source ImportSource, request ImportRequest) ImportEntry {
	entry := ImportEntry{Key: source.Key, Name: source.Name, Repairs: []Repair{}, Warnings: []Repair{}, Unsupported: map[string]string{}}
	d, canonicalErr := Parse(source.Content)
	if canonicalErr == nil {
		entry.Document = d
	} else {
		d = Document{SchemaVersion: SchemaVersion}
		fields, body, err := legacyFields(source.Content)
		d.Body = body
		if err != nil {
			entry.Repairs = append(entry.Repairs, Repair{"frontmatter", ErrInvalidDocument.Error()})
		}
		supported := map[string]bool{"schema_version": true}
		if version := fields["schema_version"]; version != nil {
			var number int
			if version.Tag != "!!int" || version.Decode(&number) != nil || number != SchemaVersion {
				entry.Repairs = append(entry.Repairs, Repair{"schemaVersion", ErrUnsupportedSchema.Error()})
			}
		}
		text := func(key string) string {
			supported[key] = true
			n := fields[key]
			if n == nil {
				return ""
			}
			if n.Kind != yaml.ScalarNode || (n.Tag != "!!str" && key != "date") {
				entry.Repairs = append(entry.Repairs, Repair{key, ErrInvalidDocument.Error()})
				return ""
			}
			return n.Value
		}
		d.ID = text("id")
		d.Session = text("session")
		d.Kind = text("kind")
		d.Title = text("title")
		d.Date = text("date")
		d.Author = text("author")
		d.State = text("state")
		d.StatusNote = text("status_note")
		d.Workstream = text("workstream")
		d.Ticket = text("ticket")
		status := text("status")
		if d.StatusNote == "" {
			d.StatusNote = status
		}
		if source.Session != nil {
			if d.Session == "" {
				d.Session = source.Session.Slug
			}
			if d.Workstream == "" {
				d.Workstream = source.Session.Workstream
			}
			if d.Ticket == "" {
				d.Ticket = source.Session.Ticket
			}
			d.Repos = append([]Repository(nil), source.Session.Repositories...)
		}
		supported["repos"] = true
		if repos := fields["repos"]; repos != nil {
			d.Repos = nil
			if repos.Kind != yaml.SequenceNode {
				entry.Repairs = append(entry.Repairs, Repair{"repos", ErrInvalidDocument.Error()})
			} else {
				for _, repo := range repos.Content {
					if repo.Kind == yaml.ScalarNode && repo.Tag == "!!str" {
						identity := request.RepositoryAliases[repo.Value]
						if identity == "" && ValidRepository(repo.Value) {
							identity = repo.Value
						}
						found := false
						if source.Session != nil {
							for _, known := range source.Session.Repositories {
								if known.Path == identity {
									d.Repos = append(d.Repos, known)
									found = true
									break
								}
							}
						}
						if !found {
							entry.Repairs = append(entry.Repairs, Repair{"repos", ErrImportRepository.Error()})
						}
					} else if checkShape(repo, map[string]string{"path": "!!str", "role": "!!str", "revision": "revision"}) == nil && len(repo.Content) == 6 {
						var r Repository
						if repo.Decode(&r) == nil {
							if alias := request.RepositoryAliases[r.Path]; alias != "" {
								r.Path = alias
							}
							d.Repos = append(d.Repos, r)
						}
					} else {
						entry.Repairs = append(entry.Repairs, Repair{"repos", ErrInvalidDocument.Error()})
					}
				}
			}
		}
		supported["lineage"] = true
		if lineage := fields["lineage"]; lineage != nil {
			if lineage.Kind != yaml.SequenceNode {
				entry.Repairs = append(entry.Repairs, Repair{"lineage", ErrInvalidDocument.Error()})
			} else {
				for _, node := range lineage.Content {
					if checkShape(node, map[string]string{"relation": "!!str", "target": "!!str"}) != nil || len(node.Content) != 4 {
						entry.Repairs = append(entry.Repairs, Repair{"lineage", ErrInvalidDocument.Error()})
						continue
					}
					var edge Edge
					if node.Decode(&edge) == nil {
						d.Lineage = append(d.Lineage, edge)
					}
				}
			}
		}
		for _, key := range []string{"research", "spec", "plan", "derived_from", "supersedes"} {
			supported[key] = true
			n := fields[key]
			if n == nil {
				continue
			}
			relation := "derived_from"
			if key == "supersedes" {
				relation = key
			}
			nodes := []*yaml.Node{n}
			if n.Kind == yaml.SequenceNode {
				nodes = n.Content
			}
			for _, node := range nodes {
				if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
					entry.Repairs = append(entry.Repairs, Repair{"lineage", ErrInvalidDocument.Error()})
					continue
				}
				d.Lineage = append(d.Lineage, Edge{Relation: relation, Target: node.Value})
			}
		}
		for key, node := range fields {
			if !supported[key] {
				raw, _ := yaml.Marshal(node)
				entry.Unsupported[key] = string(raw)
				entry.Warnings = append(entry.Warnings, Repair{key, importUnsupportedField})
			}
		}
		name := strings.TrimSuffix(filepath.Base(source.Name), filepath.Ext(source.Name))
		match := legacyFilename.FindStringSubmatch(name)
		if match != nil {
			if d.ID == "" {
				d.ID = match[1]
			}
			if d.Date == "" {
				d.Date = match[2]
			}
			if d.Kind == "" {
				d.Kind = map[byte]string{'R': "research", 'S': "spec", 'P': "plan", 'N': "note", 'D': "dead_end"}[match[1][0]]
			}
		}
		if d.Title == "" {
			for _, line := range strings.Split(d.Body, "\n") {
				if strings.HasPrefix(line, "# ") {
					d.Title = strings.TrimSpace(line[2:])
					break
				}
			}
		}
		entry.Document = d
	}
	override := request.Overrides[source.Key]
	if override.Document.SchemaVersion != 0 {
		body := entry.Document.Body
		entry.Document = override.Document
		entry.Document.Body = body
		entry.Repairs = nil
	}
	if override.Body != nil {
		entry.Document.Body = *override.Body
	}
	d = entry.Document
	if mapped := request.Namespaces[d.Session]; mapped != "" {
		old := d.Session
		d.Session = mapped
		if strings.HasPrefix(d.ID, old+"/") {
			d.ID = mapped + strings.TrimPrefix(d.ID, old)
		}
	}
	if !strings.Contains(d.ID, "/") && d.Session != "" {
		d.ID = d.Session + "/" + d.ID
	}
	entry.Document = d
	for field, value := range map[string]string{"id": d.ID, "session": d.Session, "kind": d.Kind, "title": d.Title, "date": d.Date, "author": d.Author, "state": d.State, "workstream": d.Workstream} {
		if strings.TrimSpace(value) == "" {
			entry.Repairs = append(entry.Repairs, Repair{field, importRequiredField})
		}
	}
	return entry
}
func legacyFields(data []byte) (map[string]*yaml.Node, string, error) {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	fields := map[string]*yaml.Node{}
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return fields, string(normalized), nil
	}
	end := bytes.Index(normalized[4:], []byte("\n---\n"))
	if end < 0 {
		return fields, string(normalized), ErrInvalidDocument
	}
	body := string(normalized[4+end+5:])
	if strings.TrimSpace(string(normalized[4:4+end])) == "" {
		return fields, body, nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(normalized[4 : 4+end]))
	var node yaml.Node
	if decoder.Decode(&node) != nil || len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode || uniqueKeys(node.Content[0]) != nil {
		return fields, body, ErrInvalidDocument
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return fields, body, ErrInvalidDocument
	}
	return nodeFields(node.Content[0]), body, nil
}
func insideRoot(root, path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	relative, err := filepath.Rel(canonicalProfileRoot(root), canonicalProfileRoot(path))
	return err == nil && (relative == "." || filepath.IsLocal(relative))
}
func (s *Service) profile(id string) Profile {
	for _, profile := range s.profiles {
		if profile.ID == id {
			return profile
		}
	}
	return Profile{}
}
func storeJSON(root *os.Root, path string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return ErrImportState
	}
	if err := replaceRoot(root, path, b); err != nil {
		return fmt.Errorf("%w: durable record", ErrImportState)
	}
	return nil
}
func loadImportRecord(root *os.Root, id string) (importRecord, error) {
	var record importRecord
	if !componentPattern.MatchString(id) {
		return record, ErrImportSource
	}
	b, err := readRegular(root, id+".json", maxImportRecordBytes)
	if err != nil {
		return record, ErrImportState
	}
	if err := json.Unmarshal(b, &record); err != nil || record.Version != 1 || record.Preview.ID != id {
		return record, ErrImportState
	}
	for index := range record.Sources {
		source := &record.Sources[index]
		if key := record.SourceSessions[source.Key]; key != "" {
			session, ok := record.Sessions[key]
			if !ok || session.Key != key || contentHash(string(session.Content)) != record.Hashes[key] {
				return record, ErrImportState
			}
			source.Session = &session
		}
	}
	return record, nil
}

func metadataRepairs(document Document) []Repair {
	repairs := []Repair{}
	if !validID(document.ID) {
		repairs = append(repairs, Repair{"id", importIdentityRequired})
	}
	if !componentPattern.MatchString(document.Session) || !strings.HasPrefix(document.ID, document.Session+"/") {
		repairs = append(repairs, Repair{"session", importNamespaceRequired})
	}
	if _, err := time.Parse("2006-01-02", document.Date); err != nil {
		repairs = append(repairs, Repair{"date", importDateRequired})
	}
	if document.Kind != "note" && len(document.Repos) == 0 {
		repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
	}
	for _, repo := range document.Repos {
		if !ValidRepository(repo.Path) || (repo.Role != "editing" && repo.Role != "reference") || (repo.Revision == nil && !allowsUnpinned(document)) || (repo.Revision != nil && !revisionPattern.MatchString(*repo.Revision)) {
			repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
		}
	}
	return repairs
}

func importGraph(inv inventory) map[string]Document {
	graph := map[string]Document{}
	for _, entry := range inv.entries {
		if entry.Supported && !inv.conflicts[entry.Document.ID] {
			graph[entry.Document.ID] = entry.Document
		}
	}
	return graph
}
func encodeImportRecord(record importRecord, limit int) ([]byte, error) {
	record.Sources = append([]ImportSource(nil), record.Sources...)
	record.Sessions = map[string]ImportSession{}
	record.SourceSessions = map[string]string{}
	for index := range record.Sources {
		source := &record.Sources[index]
		if source.Session != nil {
			record.Sessions[source.Session.Key] = *source.Session
			record.SourceSessions[source.Key] = source.Session.Key
			source.Session = nil
		}
	}
	encoded, err := json.Marshal(record)
	if err != nil || len(encoded) > limit {
		return nil, ErrImportSource
	}
	return encoded, nil
}

func finalizeImportPreview(preview *ImportPreview, closure bool) {
	byKey := map[string]int{}
	reverse := map[string][]string{}
	queue := []string{}
	for index, entry := range preview.Entries {
		byKey[entry.Key] = index
		for _, dependency := range entry.Dependencies {
			reverse[dependency] = append(reverse[dependency], entry.Key)
		}
		if len(entry.Repairs) > 0 || entry.Disposition == "excluded" {
			queue = append(queue, entry.Key)
		}
	}
	if closure {
		for head := 0; head < len(queue); head++ {
			for _, dependent := range reverse[queue[head]] {
				entry := &preview.Entries[byKey[dependent]]
				if len(entry.Repairs) > 0 || entry.Disposition == "excluded" {
					continue
				}
				entry.Repairs = append(entry.Repairs, Repair{"dependencies", corpusDependencyRepair})
				queue = append(queue, dependent)
			}
		}
	}
	preview.Accepted = 0
	preview.RepairRequired = 0
	preview.Excluded = 0
	for index := range preview.Entries {
		entry := &preview.Entries[index]
		if entry.Disposition == "excluded" {
			preview.Excluded++
			continue
		}
		if len(entry.Repairs) > 0 {
			entry.Disposition = "repair"
			preview.RepairRequired++
		} else {
			entry.Disposition = "ready"
			preview.Accepted++
		}
	}
}
