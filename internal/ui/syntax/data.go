package syntax

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"unicode"
)

// HighlightJSON colorizes a JSON document, distinguishing object keys, string
// values, numbers and the literals true/false/null. Whitespace and structural
// punctuation are emitted verbatim so wrapping and indentation are preserved.
func HighlightJSON(s string) string {
	runes := []rune(s)
	n := len(runes)
	var b strings.Builder
	i := 0

	for i < n {
		r := runes[i]

		switch {
		case r == '"':
			start := i
			i++
			for i < n {
				if runes[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if runes[i] == '"' {
					i++
					break
				}
				i++
			}
			str := string(runes[start:i])
			// A string is a key when the next non-space rune is a colon.
			j := i
			for j < n && (runes[j] == ' ' || runes[j] == '\t') {
				j++
			}
			if j < n && runes[j] == ':' {
				b.WriteString(StyleIdentifier.Render(str))
			} else {
				b.WriteString(StyleString.Render(str))
			}

		case unicode.IsDigit(r) || (r == '-' && i+1 < n && unicode.IsDigit(runes[i+1])):
			start := i
			i++
			for i < n {
				c := runes[i]
				if unicode.IsDigit(c) || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
					i++
					continue
				}
				break
			}
			b.WriteString(StyleNumber.Render(string(runes[start:i])))

		case unicode.IsLetter(r):
			start := i
			for i < n && unicode.IsLetter(runes[i]) {
				i++
			}
			word := string(runes[start:i])
			switch word {
			case "true", "false", "null":
				b.WriteString(StyleKeyword.Render(word))
			default:
				b.WriteString(StyleDefault.Render(word))
			}

		default:
			b.WriteRune(r)
			i++
		}
	}

	return b.String()
}

// FormatXML re-indents an XML document for readability. Invalid XML is returned
// unchanged so highlighting still applies to the raw text.
func FormatXML(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s
	}

	dec := xml.NewDecoder(strings.NewReader(trimmed))
	dec.Strict = false

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return s
		}
		if err := enc.EncodeToken(tok); err != nil {
			return s
		}
	}
	if err := enc.Flush(); err != nil {
		return s
	}

	out := buf.String()
	if strings.TrimSpace(out) == "" {
		return s
	}
	return out
}

// HighlightXML colorizes an XML document: element names, attribute names and
// their quoted values, plus comments/declarations. Text nodes are left plain.
func HighlightXML(s string) string {
	runes := []rune(s)
	n := len(runes)
	var b strings.Builder
	i := 0

	for i < n {
		if runes[i] == '<' {
			// Consume up to the matching '>'.
			start := i
			for i < n && runes[i] != '>' {
				i++
			}
			if i < n {
				i++ // include '>'
			}
			b.WriteString(highlightXMLTag(string(runes[start:i])))
		} else {
			start := i
			for i < n && runes[i] != '<' {
				i++
			}
			b.WriteString(string(runes[start:i])) // text node, plain
		}
	}

	return b.String()
}

func highlightXMLTag(tag string) string {
	// Comments, processing instructions and declarations render as comments.
	if strings.HasPrefix(tag, "<!--") || strings.HasPrefix(tag, "<?") || strings.HasPrefix(tag, "<!") {
		return StyleComment.Render(tag)
	}

	rs := []rune(tag)
	n := len(rs)
	var b strings.Builder

	i := 0
	b.WriteByte('<')
	i = 1
	if i < n && rs[i] == '/' {
		b.WriteByte('/')
		i++
	}

	// Element name.
	nameStart := i
	for i < n && isNameRune(rs[i]) {
		i++
	}
	if i > nameStart {
		b.WriteString(StyleKeyword.Render(string(rs[nameStart:i])))
	}

	// Attributes and the closing bracket.
	for i < n {
		r := rs[i]
		switch {
		case r == '"' || r == '\'':
			q := r
			start := i
			i++
			for i < n && rs[i] != q {
				i++
			}
			if i < n {
				i++
			}
			b.WriteString(StyleString.Render(string(rs[start:i])))
		case isNameRune(r):
			start := i
			for i < n && isNameRune(rs[i]) {
				i++
			}
			b.WriteString(StyleType.Render(string(rs[start:i])))
		default:
			b.WriteRune(r)
			i++
		}
	}

	return b.String()
}

func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == ':' || r == '-' || r == '_' || r == '.'
}
