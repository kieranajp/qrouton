package vault

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

type FieldEvidence struct {
	Source string `json:"source"`
	Detail string `json:"detail"`
}
type CorpusSource struct {
	Source       ImportSource `json:"source"`
	RelativePath string       `json:"relativePath"`
}
type CorpusDefaults struct {
	Author                string `json:"author"`
	State                 string `json:"state"`
	Workstream            string `json:"workstream"`
	WorkstreamFromSession bool   `json:"workstreamFromSession"`
}
type CorpusRequest struct {
	Sources             []CorpusSource
	TargetProfile       string
	Defaults            CorpusDefaults
	RepositoryAliases   map[string]string
	SessionRepositories map[string][]Repository
	ResolveRevision     func(repository, recorded string) (string, error)
	Overrides           map[string]ImportOverride
	Namespaces          map[string]string
	LinkMappings        map[string]string
	peerRepositories    map[string][]Repository
}
type CorpusEntrySummary struct {
	Key              string                   `json:"key"`
	Name             string                   `json:"name"`
	ID               string                   `json:"id"`
	Session          string                   `json:"session"`
	Kind             string                   `json:"kind"`
	Title            string                   `json:"title"`
	Destination      string                   `json:"destination"`
	Disposition      string                   `json:"disposition"`
	Reason           string                   `json:"reason,omitempty"`
	Dependencies     []string                 `json:"dependencies,omitempty"`
	Repairs          []Repair                 `json:"repairs"`
	Warnings         []Repair                 `json:"warnings"`
	UnknownRevisions bool                     `json:"unknownRevisions"`
	Provenance       map[string]FieldEvidence `json:"provenance,omitempty"`
}
type CorpusPreview struct {
	ID             string               `json:"id"`
	TargetProfile  string               `json:"targetProfile"`
	Total          int                  `json:"total"`
	Ready          int                  `json:"ready"`
	RepairRequired int                  `json:"repairRequired"`
	Excluded       int                  `json:"excluded"`
	Entries        []CorpusEntrySummary `json:"entries"`
}

func (s *Service) PreviewCorpus(ctx context.Context, request CorpusRequest) (CorpusPreview, error) {
	if len(request.Sources) == 0 || len(request.Sources) > 4096 {
		return CorpusPreview{}, ErrImportSource
	}
	if s.profile(request.TargetProfile).ID == "" {
		return CorpusPreview{}, ErrScope
	}
	total := 0
	manifests := map[string]bool{}
	for _, input := range request.Sources {
		if len(input.Source.Content) > maxDocumentBytes {
			return CorpusPreview{}, ErrImportSource
		}
		total += len(input.Source.Content)
		if session := input.Source.Session; session != nil && !manifests[session.Key] {
			manifests[session.Key] = true
			total += len(session.Content)
		}
		if total > 64<<20 {
			return CorpusPreview{}, ErrImportSource
		}
	}
	request.peerRepositories = corpusPeerRepositories(request)
	sessionEvidence := map[string][]string{}
	for _, input := range request.Sources {
		parts := strings.Split(filepath.ToSlash(input.RelativePath), "/")
		if len(parts) >= 4 && parts[1] == "shared" {
			sessionEvidence[parts[0]] = append(sessionEvidence[parts[0]], input.Source.Key)
		}
	}
	prepared := ImportRequest{prepared: map[string]ImportEntry{}, targetProfile: request.TargetProfile, LinkMappings: request.LinkMappings, Namespaces: request.Namespaces}
	seen := map[string]bool{}
	for _, input := range request.Sources {
		if input.Source.Key == "" || seen[input.Source.Key] {
			return CorpusPreview{}, ErrImportSource
		}
		seen[input.Source.Key] = true
		if err := ctx.Err(); err != nil {
			return CorpusPreview{}, err
		}
		if !filepath.IsLocal(input.RelativePath) || strings.Contains(input.RelativePath, "\\") {
			return CorpusPreview{}, ErrImportSource
		}
		source := input.Source
		source.RelativePath = filepath.ToSlash(input.RelativePath)
		if source.Name == "" {
			source.Name = path.Base(source.RelativePath)
		}
		entry := inferCorpusEntry(source, request)
		for _, evidence := range entry.Provenance {
			if evidence.Source == "session_artifacts" {
				entry.EvidenceKeys = append([]string(nil), sessionEvidence[strings.Split(source.RelativePath, "/")[0]]...)
				sort.Strings(entry.EvidenceKeys)
				break
			}
		}
		prepared.Sources = append(prepared.Sources, source)
		prepared.prepared[source.Key] = entry
	}
	preview, err := s.PreviewImport(ctx, prepared)
	if err != nil {
		return CorpusPreview{}, err
	}
	return corpusSummary(preview, request.TargetProfile), nil
}
func (s *Service) CorpusEntry(previewID, key string) (ImportEntry, error) {
	directory, err := s.importDirectory()
	if err != nil {
		return ImportEntry{}, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ImportEntry{}, ErrImportState
	}
	defer root.Close()
	record, err := loadImportRecord(root, previewID)
	if err != nil {
		return ImportEntry{}, err
	}
	for _, entry := range record.Preview.Entries {
		if entry.Key == key {
			entry.Document.Body = entry.Body
			return entry, nil
		}
	}
	return ImportEntry{}, ErrImportSource
}
func corpusSummary(preview ImportPreview, target string) CorpusPreview {
	out := CorpusPreview{ID: preview.ID, TargetProfile: target, Total: len(preview.Entries), Ready: preview.Accepted, RepairRequired: preview.RepairRequired, Excluded: preview.Excluded, Entries: []CorpusEntrySummary{}}
	for _, entry := range preview.Entries {
		out.Entries = append(out.Entries, CorpusEntrySummary{Key: entry.Key, Name: entry.Name, ID: entry.Document.ID, Session: entry.Document.Session, Kind: entry.Document.Kind, Title: entry.Document.Title, Destination: entry.Destination, Disposition: entry.Disposition, Reason: entry.Reason, Dependencies: entry.Dependencies, Repairs: entry.Repairs, Warnings: entry.Warnings, UnknownRevisions: unknownLegacyHistory(entry.Document), Provenance: entry.Provenance})
	}
	return out
}
func (s *Service) classifyCorpusDestination(entry *ImportEntry, target string) {
	for _, repair := range entry.Repairs {
		if repair.Field == "repos" {
			return
		}
	}
	if entry.Destination != "" && entry.Destination != target {
		entry.Disposition = "excluded"
		entry.Reason = corpusOtherVault
		return
	}
	if entry.Destination == target {
		return
	}
	if len(entry.Document.Repos) == 0 {
		return
	}
	knownTarget, other := false, false
	for _, repo := range entry.Document.Repos {
		if !ValidRepository(repo.Path) {
			return
		}
		org := strings.Split(repo.Path, "/")[1]
		mapped := ""
		for key, value := range s.mappings {
			if strings.EqualFold(key, org) {
				mapped = value
			}
		}
		if mapped == target {
			knownTarget = true
		} else {
			other = true
		}
	}
	if other && !knownTarget {
		entry.Disposition = "excluded"
		entry.Reason = corpusOtherOrganisation
	}
}
func inferCorpusEntry(source ImportSource, request CorpusRequest) ImportEntry {
	entry := normalizeSource(source, ImportRequest{})
	fields, _, fieldErr := legacyFields(source.Content)
	canonical, canonicalErr := Parse(source.Content)
	if canonicalErr == nil {
		entry.Document = canonical
	}
	d := entry.Document
	entry.Provenance = map[string]FieldEvidence{}
	evidence := func(field, kind, detail string) { entry.Provenance[field] = FieldEvidence{kind, detail} }
	scalar := func(key string) string {
		if node := fields[key]; node != nil && node.Kind == yaml.ScalarNode {
			return node.Value
		}
		return ""
	}
	for _, key := range []string{"id", "session", "kind", "title", "date", "author", "state", "workstream", "ticket"} {
		if scalar(key) != "" {
			evidence(key, "frontmatter", key)
		}
	}
	parts := strings.Split(source.RelativePath, "/")
	structured := len(parts) >= 4 && parts[1] == "shared" && componentPattern.MatchString(parts[0])
	folderKind := ""
	if structured {
		folderKind = map[string]string{"research": "research", "specs": "spec", "plans": "plan"}[parts[2]]
		structured = folderKind != ""
	}
	if d.Session == "" && structured {
		d.Session = parts[0]
		evidence("session", "folder", parts[0])
	}
	if structured && d.Session != "" && d.Session != parts[0] {
		entry.Repairs = append(entry.Repairs, Repair{"session", corpusSessionMismatch})
	}
	if d.Kind == "" {
		if kind := scalar("type"); kind == "research-questions" {
			d.Kind = "research"
			evidence("kind", "frontmatter", "type")
		} else if kind == "research" || kind == "spec" || kind == "plan" || kind == "note" || kind == "dead_end" {
			d.Kind = kind
			evidence("kind", "frontmatter", "type")
		} else if structured {
			d.Kind = folderKind
			evidence("kind", "folder", parts[2])
		}
	}
	if scalar("kind") == "" && d.Kind != "" {
		if _, ok := entry.Provenance["kind"]; !ok {
			evidence("kind", "filename", source.Name)
		}
	}
	if !strings.Contains(d.ID, "/") && d.Session != "" && d.ID != "" {
		d.ID = d.Session + "/" + d.ID
	}
	if scalar("id") == "" && d.ID != "" {
		evidence("id", "filename", source.Name)
	}
	if d.Date == "" {
		if date := corpusFilenameDate.FindString(source.Name); date != "" {
			d.Date = date
		}
	}
	if scalar("date") == "" && d.Date != "" {
		evidence("date", "filename", source.Name)
	}
	if scalar("title") == "" && d.Title != "" {
		evidence("title", "heading", corpusHeadingEvidence)
	}
	if d.Author == "" && request.Defaults.Author != "" {
		d.Author = request.Defaults.Author
		evidence("author", "batch_default", corpusAuthorDefault)
	}
	if d.State == "" && request.Defaults.State != "" {
		d.State = request.Defaults.State
		evidence("state", "batch_default", corpusStateDefault)
	}
	if d.Workstream == "" {
		if request.Defaults.Workstream != "" {
			d.Workstream = request.Defaults.Workstream
			evidence("workstream", "batch_default", corpusWorkstreamDefault)
		} else if request.Defaults.WorkstreamFromSession && d.Session != "" {
			d.Workstream = d.Session
			evidence("workstream", "batch_default", corpusSessionWorkstream)
		}
	}
	if source.Session != nil {
		for _, key := range []string{"session", "workstream", "ticket"} {
			if scalar(key) == "" && ((key == "session" && source.Session.Slug != "") || (key == "workstream" && source.Session.Workstream != "") || (key == "ticket" && source.Session.Ticket != "")) {
				evidence(key, "manifest", key)
			}
		}
	}
	if canonicalErr != nil {
		repos, repairs, repoEvidence, warnings := inferCorpusRepositories(fields, source, request)
		d.Repos = repos
		retained := []Repair{}
		for _, repair := range entry.Repairs {
			if repair.Field != "repos" && repair.Reason != importRequiredField {
				retained = append(retained, repair)
			}
		}
		entry.Repairs = append(retained, repairs...)
		entry.Warnings = append(entry.Warnings, warnings...)
		for key, value := range repoEvidence {
			entry.Provenance[key] = value
		}
		d.LegacyImport = &LegacyImport{SourceHash: contentHash(string(source.Content))}
		evidence("legacyImport", "source_hash", d.LegacyImport.SourceHash)
	}
	for _, key := range []string{"repo", "repository", "repositories", "type", "revision", "git_commit", "commit", "baseline", "base"} {
		if _, ok := fields[key]; ok {
			delete(entry.Unsupported, key)
			kept := entry.Warnings[:0]
			for _, warning := range entry.Warnings {
				if warning.Field != key {
					kept = append(kept, warning)
				}
			}
			entry.Warnings = kept
		}
	}
	override := request.Overrides[source.Key]
	if override.Document.SchemaVersion != 0 {
		body := d.Body
		d = override.Document
		d.Body = body
		entry.Repairs = nil
		evidence("metadata", "user_override", corpusMetadataRepair)
		if canonicalErr != nil {
			d.LegacyImport = &LegacyImport{SourceHash: contentHash(string(source.Content))}
		}
	}
	if override.Body != nil {
		d.Body = *override.Body
		evidence("body", "user_override", corpusBodyRepair)
	}
	if mapped := request.Namespaces[d.Session]; mapped != "" {
		old := d.Session
		d.Session = mapped
		if strings.HasPrefix(d.ID, old+"/") {
			d.ID = mapped + strings.TrimPrefix(d.ID, old)
		}
		evidence("session", "user_override", corpusNamespaceRepair)
	}
	if fieldErr != nil && len(entry.Repairs) == 0 {
		entry.Repairs = append(entry.Repairs, Repair{"frontmatter", ErrInvalidDocument.Error()})
	}
	if unknownLegacyHistory(d) {
		entry.Warnings = append(entry.Warnings, Repair{"repos", corpusUnknownHistory})
	}
	entry.Document = d
	return entry
}

var corpusFilenameDate = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}`)
var recordedRevision = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
var annotatedRevision = regexp.MustCompile(`\(([a-fA-F0-9]{7,64})\)`)

func inferCorpusRepositories(fields map[string]*yaml.Node, source ImportSource, request CorpusRequest) ([]Repository, []Repair, map[string]FieldEvidence, []Repair) {
	repos := []Repository{}
	repairs := []Repair{}
	warnings := []Repair{}
	evidence := map[string]FieldEvidence{}
	type candidate struct{ raw, role, revision, field string }
	candidates := []candidate{}
	declared := false
	var collect func(*yaml.Node, string)
	collect = func(node *yaml.Node, field string) {
		if node == nil {
			return
		}
		switch node.Kind {
		case yaml.ScalarNode:
			if node.Tag != "!!str" {
				repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
				return
			}
			candidates = append(candidates, candidate{raw: node.Value, field: field})
		case yaml.SequenceNode:
			for i, item := range node.Content {
				collect(item, fmt.Sprintf("%s[%d]", field, i))
			}
		case yaml.MappingNode:
			values := nodeFields(node)
			for _, key := range []string{"path", "name", "repository", "repo", "role"} {
				if v := values[key]; v != nil && (v.Kind != yaml.ScalarNode || v.Tag != "!!str") {
					repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
					return
				}
			}
			identities := map[string]bool{}
			for _, key := range []string{"path", "name", "repository", "repo"} {
				if v := values[key]; v != nil {
					identity := corpusRepositoryIdentity(v.Value, request.RepositoryAliases)
					if identity == "" {
						repairs = append(repairs, Repair{"repos", corpusUnresolvedRepository + v.Value})
						return
					}
					identities[identity] = true
				}
			}
			if len(identities) > 1 {
				repairs = append(repairs, Repair{"repos", corpusAliasConflict})
				return
			}
			get := func(key string) string {
				if n := values[key]; n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
					return n.Value
				}
				return ""
			}
			raw := get("path")
			if raw == "" {
				raw = get("name")
			}
			if raw == "" {
				raw = get("repository")
			}
			if raw == "" {
				raw = get("repo")
			}
			if raw == "" {
				if len(values) > 0 {
					for key, value := range values {
						if key != "source" && key != "target" {
							repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
							return
						}
						collect(value, field+"."+key)
					}
				}
				return
			}
			rev := get("revision")
			if value := values["revision"]; value != nil && value.Tag != "!!str" && value.Tag != "!!null" {
				repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
			}
			candidates = append(candidates, candidate{raw: raw, role: get("role"), revision: rev, field: field})
		default:
			repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
		}
	}
	for _, key := range []string{"repo", "repository", "repos", "repositories"} {
		if node := fields[key]; node != nil {
			declared = true
			collect(node, key)
		}
	}
	if !declared && source.Session != nil {
		for _, repo := range source.Session.Repositories {
			rev := ""
			if repo.Revision != nil {
				rev = *repo.Revision
			}
			candidates = append(candidates, candidate{repo.Path, repo.Role, rev, "manifest"})
		}
	}
	if !declared && len(candidates) == 0 {
		namespace := strings.Split(source.RelativePath, "/")[0]
		for _, repo := range request.SessionRepositories[namespace] {
			pin := ""
			if repo.Revision != nil {
				pin = *repo.Revision
			}
			candidates = append(candidates, candidate{repo.Path, repo.Role, pin, "session_repositories"})
		}
	}
	if !declared && len(candidates) == 0 {
		for _, repo := range request.peerRepositories[strings.Split(source.RelativePath, "/")[0]] {
			candidates = append(candidates, candidate{repo.Path, "reference", "", "session_artifacts"})
		}
	}
	topPins := map[string]bool{}
	for _, key := range []string{"revision", "git_commit", "commit", "baseline", "base"} {
		if node := fields[key]; node != nil && node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
			if recordedRevision.MatchString(node.Value) {
				topPins[node.Value] = true
			} else if key == "revision" || key == "git_commit" || key == "commit" {
				repairs = append(repairs, Repair{"repos", corpusInvalidRevision})
			}
		}
	}
	identities := map[string]bool{}
	for _, c := range candidates {
		identities[corpusRepositoryIdentity(c.raw, request.RepositoryAliases)] = true
	}
	if len(topPins) > 0 && len(identities) != 1 {
		repairs = append(repairs, Repair{"repos", corpusRevisionConflict})
	}
	resolvePin := func(identity, pin string) string {
		pin = strings.ToLower(pin)
		if !revisionPattern.MatchString(pin) && request.ResolveRevision != nil {
			if full, err := request.ResolveRevision(identity, pin); err == nil && revisionPattern.MatchString(full) {
				return strings.ToLower(full)
			}
		}
		return pin
	}
	seen := map[string]int{}
	for _, c := range candidates {
		raw := strings.TrimSpace(c.raw)
		if c.revision == "" {
			if matches := annotatedRevision.FindAllStringSubmatch(raw, -1); len(matches) == 1 {
				c.revision = matches[0][1]
			}
		}
		if c.role == "" {
			lower := strings.ToLower(raw)
			if strings.Contains(lower, "(reference") {
				c.role = "reference"
			} else if strings.Contains(lower, "(active") || strings.Contains(lower, "(editing") {
				c.role = "editing"
			}
		}
		identity := corpusRepositoryIdentity(raw, request.RepositoryAliases)
		if identity == "" {
			repairs = append(repairs, Repair{"repos", corpusUnresolvedRepository + raw})
			continue
		}
		origin := "frontmatter"
		if c.field == "manifest" {
			origin = "manifest"
		} else if c.field == "session_repositories" {
			origin = "batch_default"
		} else if c.field == "session_artifacts" {
			origin = "session_artifacts"
		}
		roleOrigin, revisionOrigin := origin, origin
		if len(topPins) > 0 && len(identities) == 1 {
			explicit := map[string]bool{}
			for pin := range topPins {
				explicit[resolvePin(identity, pin)] = true
			}
			if c.revision != "" && c.field != "manifest" && c.field != "session_repositories" {
				explicit[resolvePin(identity, c.revision)] = true
			}
			if len(explicit) != 1 {
				repairs = append(repairs, Repair{"repos", corpusRevisionConflict})
			} else {
				for pin := range explicit {
					c.revision = pin
				}
				revisionOrigin = "frontmatter"
			}
		}
		if source.Session != nil {
			for _, repo := range source.Session.Repositories {
				if repo.Path != identity {
					continue
				}
				if c.role == "" {
					c.role = repo.Role
					roleOrigin = "manifest"
				}
				if c.revision == "" && repo.Revision != nil {
					c.revision = *repo.Revision
					revisionOrigin = "manifest"
					evidence[identity+".revision"] = FieldEvidence{"manifest", corpusManifestRevision}
				}
			}
		}
		if c.role == "active" {
			c.role = "editing"
		}
		if strings.HasPrefix(c.role, "reference ") {
			c.role = "reference"
		}
		if c.role == "" {
			c.role = "reference"
			evidence[identity+".role"] = FieldEvidence{"batch_default", corpusReferenceDefault}
			warnings = append(warnings, Repair{"repos", corpusReferenceWarning + identity})
		} else {
			evidence[identity+".role"] = FieldEvidence{roleOrigin, c.field}
		}
		var revision *string
		if c.revision != "" {
			if !recordedRevision.MatchString(c.revision) {
				repairs = append(repairs, Repair{"repos", corpusInvalidRevision})
			} else if revisionPattern.MatchString(c.revision) {
				pin := c.revision
				revision = &pin
				evidence[identity+".revision"] = FieldEvidence{revisionOrigin, corpusFullCommit}
			} else if request.ResolveRevision != nil {
				if full, err := request.ResolveRevision(identity, c.revision); err == nil && revisionPattern.MatchString(full) {
					revision = &full
					evidence[identity+".revision"] = FieldEvidence{"local_git", corpusExpandedCommit + c.revision}
				} else {
					warnings = append(warnings, Repair{"repos", corpusUnresolvedCommit + c.revision})
				}
			}
		}
		if revision == nil {
			evidence[identity+".revision"] = FieldEvidence{"legacy_unknown", corpusNoRevision}
		}
		evidence[identity] = FieldEvidence{origin, c.field}
		repo := Repository{Path: identity, Role: c.role, Revision: revision}
		if index, exists := seen[identity]; exists {
			prior := repos[index]
			if prior.Role != repo.Role || (prior.Revision == nil) != (repo.Revision == nil) || (prior.Revision != nil && repo.Revision != nil && *prior.Revision != *repo.Revision) {
				repairs = append(repairs, Repair{"repos", corpusRepositoryConflict + identity})
			}
			continue
		}
		seen[identity] = len(repos)
		repos = append(repos, repo)
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Path < repos[j].Path })
	if len(repos) == 0 {
		repairs = append(repairs, Repair{"repos", ErrImportRepository.Error()})
	}
	return repos, repairs, evidence, warnings
}
func corpusRepositoryIdentity(raw string, aliases map[string]string) string {
	original := raw
	raw = strings.Trim(strings.TrimSpace(raw), "`")
	raw = strings.TrimPrefix(raw, "https://github.com/")
	raw = strings.TrimPrefix(raw, "git@github.com:")
	raw = strings.TrimPrefix(raw, "github.com/")
	if i := strings.IndexAny(raw, " @(\t"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.Trim(raw, "`,")
	raw = strings.TrimSuffix(raw, ".git")
	raw = strings.TrimLeft(raw, "./")
	raw = strings.TrimPrefix(raw, "src/")
	candidates := map[string]bool{}
	if ValidRepository("github.com/" + raw) {
		candidates["github.com/"+raw] = true
	}
	for _, key := range []string{original, raw, "src/" + raw} {
		if mapped, present := aliases[key]; present {
			if !ValidRepository(mapped) {
				return ""
			}
			candidates[mapped] = true
		}
	}
	if len(candidates) != 1 {
		return ""
	}
	for candidate := range candidates {
		return candidate
	}
	return ""
}

func corpusPeerRepositories(request CorpusRequest) map[string][]Repository {
	type scope struct {
		ids     string
		repos   []Repository
		blocked bool
	}
	groups := map[string]*scope{}
	for _, input := range request.Sources {
		parts := strings.Split(filepath.ToSlash(input.RelativePath), "/")
		if len(parts) < 4 || parts[1] != "shared" {
			continue
		}
		key := parts[0]
		g := groups[key]
		if g == nil {
			g = &scope{}
			groups[key] = g
		}
		admit := func(repos []Repository, invalid bool) {
			if invalid || len(repos) == 0 {
				g.blocked = true
				return
			}
			set := map[string]bool{}
			for _, repo := range repos {
				if !ValidRepository(repo.Path) {
					g.blocked = true
					return
				}
				set[repo.Path] = true
			}
			ids := []string{}
			for id := range set {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			joined := strings.Join(ids, "\n")
			if g.ids != "" && g.ids != joined {
				g.blocked = true
				return
			}
			g.ids = joined
			g.repos = nil
			for _, id := range ids {
				g.repos = append(g.repos, Repository{Path: id, Role: "reference"})
			}
		}
		fields, _, err := legacyFields(input.Source.Content)
		if err != nil {
			g.blocked = true
			continue
		}
		declared := false
		for _, name := range []string{"repo", "repository", "repos", "repositories"} {
			if fields[name] != nil {
				declared = true
			}
		}
		if declared {
			isolated := input.Source
			isolated.Session = nil
			r := request
			r.SessionRepositories = nil
			r.peerRepositories = nil
			r.ResolveRevision = nil
			repos, repairs, _, _ := inferCorpusRepositories(fields, isolated, r)
			admit(repos, len(repairs) > 0)
		}
		if input.Source.Session != nil && len(input.Source.Session.Repositories) > 0 {
			admit(input.Source.Session.Repositories, false)
		}
	}
	result := map[string][]Repository{}
	for key, g := range groups {
		if !g.blocked && g.ids != "" {
			result[key] = g.repos
		}
	}
	return result
}
