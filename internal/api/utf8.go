package api

import (
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// validStrings repairs every string m holds. proto refuses to marshal invalid
// UTF-8, and one such event would end every stream it reached.
func validStrings(m protoreflect.Message) {
	var repairs []func()
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			repairs = append(repairs, validMap(fd, v.Map())...)
		case fd.IsList():
			list := v.List()
			for i := range list.Len() {
				switch fd.Kind() {
				case protoreflect.StringKind:
					if s := list.Get(i).String(); !utf8.ValidString(s) {
						repairs = append(repairs, func() { list.Set(i, protoreflect.ValueOfString(validUTF8(s))) })
					}
				case protoreflect.MessageKind:
					validStrings(list.Get(i).Message())
				}
			}
		case fd.Kind() == protoreflect.StringKind:
			if s := v.String(); !utf8.ValidString(s) {
				repairs = append(repairs, func() { m.Set(fd, protoreflect.ValueOfString(validUTF8(s))) })
			}
		case fd.Kind() == protoreflect.MessageKind:
			validStrings(v.Message())
		}
		return true
	})
	for _, repair := range repairs {
		repair()
	}
}

// validMap repairs a map's string keys and values in place, and walks its message values.
func validMap(fd protoreflect.FieldDescriptor, m protoreflect.Map) []func() {
	var repairs []func()
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		switch fd.MapValue().Kind() {
		case protoreflect.MessageKind:
			validStrings(v.Message())
		case protoreflect.StringKind:
			if s := v.String(); !utf8.ValidString(s) {
				v = protoreflect.ValueOfString(validUTF8(s))
				repairs = append(repairs, func() { m.Set(k, v) })
			}
		}
		if fd.MapKey().Kind() == protoreflect.StringKind {
			if key := k.String(); !utf8.ValidString(key) {
				repairs = append(repairs, func() {
					m.Clear(k)
					m.Set(protoreflect.ValueOfString(validUTF8(key)).MapKey(), v)
				})
			}
		}
		return true
	})
	return repairs
}

// validUTF8 replaces each invalid byte as encoding/json does, where
// strings.ToValidUTF8 would fold a run of them into one.
func validUTF8(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			out.WriteRune(utf8.RuneError)
		} else {
			out.WriteString(s[i : i+size])
		}
		i += size
	}
	return out.String()
}
