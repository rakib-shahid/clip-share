package library

import "testing"

func TestNormalizeFolderName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		display    string
		normalized string
		valid      bool
	}{
		{name: "trims and folds", input: "  Game Clips  ", display: "Game Clips", normalized: "game clips", valid: true},
		{name: "unicode equivalent", input: "Cafe\u0301", display: "Cafe\u0301", normalized: "café", valid: true},
		{name: "slash", input: "one/two", valid: false},
		{name: "backslash", input: `one\two`, valid: false},
		{name: "dot", input: "..", valid: false},
		{name: "control", input: "one\ntwo", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			display, normalized, err := NormalizeFolderName(test.input)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
			if test.valid && (display != test.display || normalized != test.normalized) {
				t.Fatalf("got display=%q normalized=%q", display, normalized)
			}
		})
	}
}

func TestNormalizeClipTitleAllowsLongerTitles(t *testing.T) {
	display, normalized, err := NormalizeClipTitle("  Launch DAY  ")
	if err != nil || display != "Launch DAY" || normalized != "launch day" {
		t.Fatalf("display=%q normalized=%q err=%v", display, normalized, err)
	}
}

func TestNormalizeSearchQuery(t *testing.T) {
	if got := NormalizeSearchQuery("  RÉSUMÉ  "); got != "résumé" {
		t.Fatalf("normalized search query = %q", got)
	}
	if got := NormalizeSearchQuery("   "); got != "" {
		t.Fatalf("empty search query = %q", got)
	}
}
