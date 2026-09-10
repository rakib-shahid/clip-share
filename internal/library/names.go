package library

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var ErrInvalidFolderName = errors.New("folder name must be 1-100 characters and cannot be '.', '..', contain slashes, or contain control characters")
var ErrInvalidClipTitle = errors.New("clip title must be 1-200 characters and cannot be '.', '..', contain slashes, or contain control characters")

// NormalizeFolderName returns both the display value and the comparison value.
// We preserve a user's capitalization while using Unicode normalization and
// case folding to detect sibling names that look equivalent.
func NormalizeFolderName(value string) (display string, normalized string, err error) {
	return normalizeName(value, 100, ErrInvalidFolderName)
}

func NormalizeClipTitle(value string) (display string, normalized string, err error) {
	return normalizeName(value, 200, ErrInvalidClipTitle)
}

// NormalizeSearchQuery applies the same Unicode normalization and case folding
// used by stored folder names and clip titles. Search does not need validation;
// an empty normalized value simply means "no results".
func NormalizeSearchQuery(value string) string {
	return cases.Fold().String(norm.NFC.String(strings.TrimSpace(value)))
}

func normalizeName(value string, maximum int, invalidError error) (display string, normalized string, err error) {
	display = strings.TrimSpace(value)
	if display == "" || utf8.RuneCountInString(display) > maximum || display == "." || display == ".." {
		return "", "", invalidError
	}
	for _, r := range display {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return "", "", invalidError
		}
	}
	normalized = cases.Fold().String(norm.NFC.String(display))
	return display, normalized, nil
}
