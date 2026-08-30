package evalharness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Normalize(provider string, data []byte, turn int) ([]Event, string, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, streamBufferInitial), streamBufferMax)

	var events []Event
	var final string
	var session string
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}

		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return events, final, session, fmt.Errorf(
				"%s event line %d: %w",
				provider,
				lineNumber,
				err,
			)
		}
		sanitized, visible := sanitizeObservable(value)
		if !visible {
			continue
		}
		value = sanitized.(map[string]any)

		typeName := firstString(value, eventTypeKeys...)
		if id := firstString(value, sessionIDKeys...); id != "" {
			session = id
		}
		text := extractText(value)
		name := firstString(value, toolNameKeys...)
		kind := normalizedKind(typeName, name)
		if text != "" && (kind == kindAssistant || kind == kindResult) {
			final = text
		}

		arguments, err := json.Marshal(value)
		if err != nil {
			return events, final, session, fmt.Errorf("normalize %s event: %w", provider, err)
		}
		events = append(events, Event{
			Time:      time.Now().UTC(),
			Kind:      kind,
			Turn:      turn,
			Role:      provider,
			Name:      name,
			Text:      text,
			Arguments: arguments,
			RawType:   typeName,
		})
	}
	if err := scanner.Err(); err != nil {
		return events, final, session, err
	}
	if lineNumber == 0 {
		return nil, "", session, fmt.Errorf("%w: %s", ErrNoEvents, provider)
	}
	return events, final, session, nil
}

func sanitizeObservable(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		typeName, _ := typed["type"].(string)
		lowerType := strings.ToLower(typeName)
		if strings.Contains(lowerType, hiddenThinking) || strings.Contains(lowerType, hiddenReasoning) {
			return nil, false
		}
		cleaned := make(map[string]any, len(typed))
		for key, nested := range typed {
			item, visible := sanitizeObservable(nested)
			if key == itemKey && !visible {
				return nil, false
			}
			if visible {
				cleaned[key] = item
			}
		}
		return cleaned, true
	case []any:
		cleaned := make([]any, 0, len(typed))
		for _, nested := range typed {
			item, visible := sanitizeObservable(nested)
			if visible {
				cleaned = append(cleaned, item)
			}
		}
		return cleaned, true
	default:
		return value, true
	}
}

func normalizedKind(typeName, name string) string {
	value := strings.ToLower(typeName + " " + name)
	switch {
	case strings.Contains(value, "tool"), strings.Contains(value, "function"):
		return kindToolCall
	case strings.Contains(value, "agent"), strings.Contains(value, "task"):
		return delegationKind
	case strings.Contains(value, "assistant"), strings.Contains(value, "message"):
		return kindAssistant
	case strings.Contains(value, "result"), strings.Contains(value, "completed"):
		return kindResult
	default:
		return kindProviderEvent
	}
}

// firstString returns the first key holding a non-empty string: a
// present-but-empty session id or tool name is no more useful than an absent one.
func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func extractText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		for index := len(typed) - 1; index >= 0; index-- {
			if text := extractText(typed[index]); text != "" {
				return text
			}
		}
	case map[string]any:
		keys := textKeys
		for _, key := range keys {
			item, ok := typed[key]
			if !ok {
				continue
			}
			if text := extractText(item); text != "" {
				return text
			}
		}
	}
	return ""
}

func ReadMCPEvents(path string, turn int) []Event {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var events []Event
	for _, line := range strings.Split(string(content), "\n") {
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		event.Turn = turn
		events = append(events, event)
	}
	return events
}

func AppendJSONL(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), outputDirMode); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, outputFileMode)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = file.Write(append(encoded, '\n'))
	return err
}
