package vault

import (
	"html"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

var referenceDefinition = regexp.MustCompile(`^( {0,3}\[[^\]\n]+\]:[ \t]*)(<[^>\n]*>|[^ \t\n]+)(.*)$`)
var citationSuffix = regexp.MustCompile(`^(.*):([0-9]+)(?:-([0-9]+))?$`)
var linkScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

type importLinkIndex struct {
	profiles    []string
	unavailable map[string]bool
	warnings    *[]Repair
	rewrite     func(string) (string, bool)
	aliases     map[string]map[string]ImportEntry
}

func newImportLinkIndex(entries []ImportEntry, sources []ImportSource) *importLinkIndex {
	index := &importLinkIndex{aliases: map[string]map[string]ImportEntry{}}
	byKey := map[string]ImportSource{}
	for _, source := range sources {
		byKey[source.Key] = source
	}
	for _, entry := range entries {
		if !validID(entry.Document.ID) {
			continue
		}
		if !slices.Contains(index.profiles, entry.Destination) {
			index.profiles = append(index.profiles, entry.Destination)
		}
		source := byKey[entry.Key]
		add := func(alias string) {
			if alias == "" {
				return
			}
			key := entry.Destination + "\x00" + alias
			if index.aliases[key] == nil {
				index.aliases[key] = map[string]ImportEntry{}
			}
			index.aliases[key][entry.Key] = entry
		}
		add(entry.Document.ID)
		add(entry.Name)
		add(strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name)))
		add(entry.Document.Title)
		if entry.Path != "" {
			add("path:" + path.Clean(entry.Path))
		}
		if source.Origin != "" {
			add("origin:" + canonicalProfileRoot(source.Origin))
		}
		if source.RelativePath != "" {
			add("corpus:" + path.Clean(source.RelativePath))
		}
		if d, err := Parse(source.Content); err == nil {
			add(d.ID)
		} else if fields, _, err := legacyFields(source.Content); err == nil {
			if id := fields["id"]; id != nil && id.Tag == "!!str" {
				value := id.Value
				if !strings.Contains(value, "/") {
					namespace := entry.Document.Session
					if session := fields["session"]; session != nil && session.Tag == "!!str" {
						namespace = session.Value
					}
					value = namespace + "/" + value
				}
				add(value)
			}
		}
	}
	return index
}
func (index *importLinkIndex) resolve(target string, source ImportSource, current ImportEntry) (ImportEntry, bool) {
	entry, count := index.resolveCount(target, source, current)
	return entry, count == 1
}
func (index *importLinkIndex) resolveCount(target string, source ImportSource, current ImportEntry) (ImportEntry, int) {
	target = strings.SplitN(target, "#", 2)[0]
	if decoded, err := url.PathUnescape(target); err == nil {
		target = decoded
	}
	aliases := []string{target, "path:" + path.Clean(path.Join(path.Dir(current.Path), target))}
	if source.Origin != "" {
		resolved := filepath.Clean(filepath.Join(filepath.Dir(source.Origin), filepath.FromSlash(target)))
		if filepath.IsAbs(target) {
			resolved = filepath.Clean(target)
		}
		aliases = append(aliases, "origin:"+canonicalProfileRoot(resolved))
	}
	if source.RelativePath != "" {
		parts := strings.Split(source.RelativePath, "/")
		if strings.HasPrefix(target, "thoughts/shared/") {
			aliases = append(aliases, "corpus:"+parts[0]+"/"+strings.TrimPrefix(target, "thoughts/"))
		}
		if strings.HasPrefix(target, "shared/") {
			aliases = append(aliases, "corpus:"+parts[0]+"/"+target)
		}
		aliases = append(aliases, "corpus:"+path.Clean(target))
		aliases = append(aliases, "corpus:"+path.Clean(path.Join(path.Dir(source.RelativePath), target)))
	}
	candidates := map[string]ImportEntry{}
	for _, alias := range aliases[1:] {
		for key, entry := range index.aliases[current.Destination+"\x00"+alias] {
			candidates[key] = entry
		}
	}
	if len(candidates) == 1 {
		for _, entry := range candidates {
			return entry, 1
		}
	}
	for _, alias := range aliases {
		for key, entry := range index.aliases[current.Destination+"\x00"+alias] {
			candidates[key] = entry
		}
	}
	if len(candidates) != 1 {
		return ImportEntry{}, len(candidates)
	}
	for _, entry := range candidates {
		return entry, 1
	}
	return ImportEntry{}, len(candidates)
}
func importLinkTarget(target string, source ImportSource, current ImportEntry, entries []ImportEntry, sources []ImportSource) (ImportEntry, bool) {
	return newImportLinkIndex(entries, sources).resolve(target, source, current)
}
func normalizeImportLinks(body string, source ImportSource, current ImportEntry, entries []ImportEntry, sources []ImportSource, mappings map[string]string) (string, []Repair, []string) {
	return normalizeImportLinksIndexed(body, source, current, mappings, newImportLinkIndex(entries, sources))
}
func normalizeImportLinksIndexed(body string, source ImportSource, current ImportEntry, mappings map[string]string, index *importLinkIndex) (string, []Repair, []string) {
	repairs := []Repair{}
	dependencies := map[string]bool{}
	rewrite := func(destination string) (string, bool) {
		if index.rewrite != nil {
			if result, ok := index.rewrite(destination); ok {
				return result, true
			}
		}
		if result, ok := assetLink(current, destination); ok {
			return result, true
		}
		original := destination
		if mapped, ok := mappings[destination]; ok {
			destination = mapped
		}
		if destination == "" || strings.HasPrefix(destination, "#") {
			return destination, true
		}
		if linkScheme.MatchString(destination) && !strings.HasPrefix(destination, "file:") && (strings.Contains(destination, "://") || citationSuffix.FindStringSubmatch(destination) == nil) {
			return destination, true
		}
		if strings.HasPrefix(destination, "//") {
			return destination, true
		}
		target, fragment, _ := strings.Cut(destination, "#")
		sameProfile, sameProfileCount := index.resolveCount(target, source, current)
		sameProfileFound := sameProfileCount == 1
		if sameProfileCount > 1 {
			repairs = append(repairs, Repair{"body", ErrImportLink.Error() + ": " + original})
			return original, false
		}
		if index.unavailable != nil && (!sameProfileFound || index.unavailable[sameProfile.Key]) {

			for _, profile := range index.profiles {
				probe := current
				probe.Destination = profile
				if found, ok := index.resolve(target, source, probe); ok && (profile != current.Destination || index.unavailable[found.Key]) {
					if index.warnings != nil {
						*index.warnings = append(*index.warnings, Repair{"body", corpusInertLink + ": " + original})
					}
					return "\x00" + original, true
				}
			}
		}
		if entry, ok := sameProfile, sameProfileFound; ok {
			fromPath, targetPath := current.Path, entry.Path
			if fromPath == "" {
				fromPath = current.Document.ID + ".md"
			}
			if targetPath == "" {
				targetPath = entry.Document.ID + ".md"
			}
			relative, err := filepath.Rel(filepath.Dir(fromPath), targetPath)
			if err != nil {
				return original, false
			}
			result := escapeMarkdownPath(filepath.ToSlash(relative))
			if fragment != "" {
				result += "#" + fragment
			}
			if entry.Key != current.Key {
				dependencies[entry.Key] = true
			}
			return result, true
		}
		if citation, ok := normalizeCodeCitation(destination, source, current.Document); ok {
			return citation, true
		}
		repairs = append(repairs, Repair{Field: "body", Reason: ErrImportLink.Error() + ": " + original})
		return original, false
	}
	var output strings.Builder
	fence := byte(0)
	fenceWidth := 0
	inlineWidth := 0
	offset := 0
	for _, line := range strings.SplitAfter(body, "\n") {
		lineOffset := offset
		offset += len(line)
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') && inlineWidth == 0 {
			width := runWidth(trimmed, 0, trimmed[0])
			if width >= 3 && (fence == 0 || (fence == trimmed[0] && width >= fenceWidth && strings.TrimSpace(trimmed[width:]) == "")) {
				if fence == 0 {
					fence = trimmed[0]
					fenceWidth = width
				} else {
					fence = 0
				}
				output.WriteString(line)
				continue
			}
		}
		if fence != 0 || (indent >= 4 && inlineWidth == 0) {
			output.WriteString(line)
			continue
		}
		if inlineWidth == 0 {
			if match := referenceDefinition.FindStringSubmatch(strings.TrimSuffix(line, "\n")); match != nil {
				value := match[2]
				angled := strings.HasPrefix(value, "<")
				if angled {
					value = strings.TrimSuffix(value[1:], ">")
				}
				decoded := unescapeMarkdown(value)
				rewritten, _ := rewrite(decoded)
				if strings.HasPrefix(rewritten, "\x00") {
					output.WriteString(inertText(strings.TrimSuffix(line, "\n")) + "\n")
					continue
				}
				if rewritten == decoded {
					rewritten = value
				}
				if angled {
					rewritten = "<" + rewritten + ">"
				}
				output.WriteString(match[1] + rewritten + match[3])
				if strings.HasSuffix(line, "\n") {
					output.WriteByte('\n')
				}
				continue
			}
		}
		for at := 0; at < len(line); {
			if inlineWidth > 0 {
				if line[at] == '`' && runWidth(line, at, '`') == inlineWidth {
					output.WriteString(line[at : at+inlineWidth])
					at += inlineWidth
					inlineWidth = 0
				} else {
					output.WriteByte(line[at])
					at++
				}
				continue
			}
			if line[at] == '\\' && at+1 < len(line) {
				output.WriteString(line[at : at+2])
				at += 2
				continue
			}
			if line[at] == '`' {
				width := runWidth(line, at, '`')
				if hasClosingTicks(body[lineOffset+at+width:], width) {
					inlineWidth = width
				}
				output.WriteString(line[at : at+width])
				at += width
				continue
			}
			if strings.HasPrefix(line[at:], "[[") {
				end := strings.Index(line[at+2:], "]]")
				if end >= 0 {
					if at > 0 && line[at-1] == '!' {
						repairs = append(repairs, Repair{"body", ErrImportLink.Error()})
						output.WriteString(line[at : at+end+4])
						at += end + 4
						continue
					}
					inner := line[at+2 : at+2+end]
					target, label, alias := strings.Cut(inner, "|")
					if !alias {
						label = target
					}
					rewritten, ok := rewrite(target)
					if strings.HasPrefix(rewritten, "\x00") {
						output.WriteString(inertText(label + " (" + target + ")"))
					} else if ok {
						output.WriteString("[" + strings.ReplaceAll(label, "]", "\\]") + "](" + rewritten + ")")
					} else {
						output.WriteString(line[at : at+end+4])
					}
					at += end + 4
					continue
				}
			}
			if line[at] == '[' {
				close := matchingBracket(line, at, '[', ']')
				if close >= 0 && close+1 < len(line) && line[close+1] == '(' {
					end := matchingBracket(line, close+1, '(', ')')
					if end < 0 {
						repairs = append(repairs, Repair{"body", ErrImportLink.Error()})
						output.WriteString(line[at:])
						break
					}
					inner := line[close+2 : end]
					begin, finish, ok := destinationSpan(inner)
					if !ok {
						repairs = append(repairs, Repair{"body", ErrImportLink.Error()})
						output.WriteString(line[at : end+1])
					} else {
						raw := inner[begin:finish]
						decoded := unescapeMarkdown(raw)
						rewritten, _ := rewrite(decoded)
						if strings.HasPrefix(rewritten, "\x00") {
							output.WriteString(inertText(line[at+1:close] + " (" + decoded + ")"))
							at = end + 1
							continue
						}
						if rewritten == decoded {
							rewritten = raw
						}
						output.WriteString(line[at:close+2] + inner[:begin] + rewritten + inner[finish:] + ")")
					}
					at = end + 1
					continue
				}
			}
			output.WriteByte(line[at])
			at++
		}
	}
	deps := make([]string, 0, len(dependencies))
	for key := range dependencies {
		deps = append(deps, key)
	}
	sort.Strings(deps)
	return output.String(), repairs, deps
}
func runWidth(text string, at int, char byte) int {
	end := at
	for end < len(text) && text[end] == char {
		end++
	}
	return end - at
}
func matchingBracket(text string, at int, open, close byte) int {
	depth := 0
	quoted := byte(0)
	for index := at; index < len(text); index++ {
		c := text[index]
		if c == '\\' {
			index++
			continue
		}
		if open == '(' && quoted != 0 {
			if c == quoted {
				quoted = 0
			}
			continue
		}
		if open == '(' && (c == '"' || c == '\'') && index > at && unicode.IsSpace(rune(text[index-1])) {
			quoted = c
			continue
		}
		if c == open {
			depth++
		}
		if c == close {
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}
func destinationSpan(inner string) (int, int, bool) {
	begin := 0
	for begin < len(inner) && unicode.IsSpace(rune(inner[begin])) {
		begin++
	}
	if begin == len(inner) {
		return begin, begin, true
	}
	if inner[begin] == '<' {
		end := strings.IndexByte(inner[begin+1:], '>')
		if end < 0 {
			return 0, 0, false
		}
		return begin + 1, begin + 1 + end, true
	}
	end := begin
	depth := 0
	for end < len(inner) {
		c := inner[end]
		if c == '\\' && end+1 < len(inner) {
			end += 2
			continue
		}
		if unicode.IsSpace(rune(c)) && depth == 0 {
			break
		}
		if c == '(' {
			depth++
		}
		if c == ')' {
			depth--
		}
		end++
	}
	return begin, end, depth == 0
}
func unescapeMarkdown(value string) string {
	var out strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			index++
		}
		out.WriteByte(value[index])
	}
	return out.String()
}
func escapeMarkdownPath(value string) string {
	return strings.NewReplacer("%", "%25", " ", "%20", "(", "%28", ")", "%29", "#", "%23", "?", "%3F").Replace(value)
}
func normalizeCodeCitation(destination string, source ImportSource, document Document) (string, bool) {
	fragment := ""
	raw := destination
	if match := citationSuffix.FindStringSubmatch(raw); match != nil {
		raw = match[1]
		fragment = "L" + match[2]
		if match[3] != "" {
			fragment += "-L" + match[3]
		}
	} else if target, anchor, ok := strings.Cut(raw, "#"); ok {
		raw = target
		fragment = anchor
	}
	if strings.HasPrefix(raw, "file:") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		raw = parsed.Path
	}
	if decoded, err := url.PathUnescape(raw); err == nil {
		raw = decoded
	}
	candidates := map[string]bool{}
	for _, repo := range document.Repos {
		if repo.Revision == nil && !unknownLegacyHistory(document) {
			continue
		}
		prefix := "src/" + path.Base(repo.Path) + "/"
		relative := ""
		if source.RelativePath != "" && !filepath.IsAbs(raw) {
			candidate := path.Clean(path.Join(path.Dir(source.RelativePath), raw))
			if strings.HasPrefix(candidate, prefix) {
				relative = strings.TrimPrefix(candidate, prefix)
			}
		}
		if source.Session != nil && source.Session.Origin != "" {
			absolute := raw
			if !filepath.IsAbs(raw) {
				absolute = filepath.Join(filepath.Dir(source.Origin), filepath.FromSlash(raw))
			}
			base := filepath.Join(filepath.Dir(source.Session.Origin), filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
			candidate, err := filepath.Rel(canonicalProfileRoot(base), canonicalProfileRoot(absolute))
			if err == nil && filepath.IsLocal(candidate) {
				relative = filepath.ToSlash(candidate)
			}
		} else if strings.HasPrefix(raw, prefix) {
			relative = strings.TrimPrefix(raw, prefix)
		}
		if relative == "" || !filepath.IsLocal(relative) {
			continue
		}
		target := ""
		if repo.Revision == nil {
			target = "repo://" + repo.Path + "/" + escapeMarkdownPath(relative) + "?revision=unknown"
		} else {
			target = "https://" + repo.Path + "/blob/" + *repo.Revision + "/" + escapeMarkdownPath(relative)
		}
		if fragment != "" {
			target += "#" + fragment
		}
		candidates[target] = true
	}
	if len(candidates) != 1 {
		return "", false
	}
	for target := range candidates {
		return target, true
	}
	return "", false
}
func hasClosingTicks(text string, width int) bool {
	for at := 0; at < len(text); at++ {
		if text[at] != '`' {
			continue
		}
		run := runWidth(text, at, '`')
		if run == width {
			return true
		}
		at += run - 1
	}
	return false
}

func (s *Service) ResolveLink(scope Scope, from Reference, target string) (ReadResult, string, error) {
	admitted, err := s.Resolve(scope, from)
	if err != nil {
		return ReadResult{}, "", err
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || strings.Contains(parsed.Path, "\\") || strings.HasPrefix(parsed.Path, "/") {
		return ReadResult{}, "", ErrScope
	}
	relative := path.Clean(path.Join(path.Dir(admitted.Reference.Path), parsed.Path))
	if parsed.Path == "" {
		relative = admitted.Reference.Path
	}
	if !filepath.IsLocal(relative) {
		return ReadResult{}, "", ErrScope
	}
	result, err := s.Resolve(scope, Reference{Profile: admitted.Reference.Profile, Path: relative})
	return result, parsed.Fragment, err
}

func inertText(value string) string {
	value = html.EscapeString(value)
	return strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "&#96;", "<", "&lt;", ">", "&gt;", "\n", " ").Replace(value)
}
