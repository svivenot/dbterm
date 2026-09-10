package syntax

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestHighlightJSONPreservesText(t *testing.T) {
	in := `{"language":"fr","count":42,"active":true,"note":null}`
	out := HighlightJSON(in)
	if got := stripANSI(out); got != in {
		t.Fatalf("highlighting altered the text:\n in=%q\nout=%q", in, got)
	}
	// The key and its value must both survive the transformation.
	for _, want := range []string{`"language"`, `"fr"`, "42", "true", "null"} {
		if !strings.Contains(stripANSI(out), want) {
			t.Errorf("expected token %q in output", want)
		}
	}
}

func TestFormatAndHighlightXML(t *testing.T) {
	in := `<root><child id="1">value</child></root>`

	formatted := FormatXML(in)
	if !strings.Contains(formatted, "\n") {
		t.Errorf("expected FormatXML to indent onto multiple lines, got %q", formatted)
	}

	out := HighlightXML(formatted)
	stripped := stripANSI(out)
	for _, want := range []string{"<root>", `<child id="1">`, "value", "</child>", "</root>"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("expected %q in highlighted XML, got:\n%s", want, stripped)
		}
	}
}

func TestFormatXMLInvalidPassthrough(t *testing.T) {
	in := "<not valid <<< xml"
	if got := FormatXML(in); got != in {
		t.Errorf("expected invalid XML returned unchanged, got %q", got)
	}
}
