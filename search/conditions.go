package search

import (
	"mime"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	typeRegexp = regexp.MustCompile(`type:(\w+)`)
)

type condition func(path string) bool

func extensionCondition(extension string) condition {
	return func(path string) bool {
		// A camera's .JPG or a scanner's .PDF is of its type too (Gezgin).
		return strings.EqualFold(filepath.Ext(path), "."+extension)
	}
}

func imageCondition(path string) bool {
	extension := filepath.Ext(path)
	mimetype := mime.TypeByExtension(extension)

	return strings.HasPrefix(mimetype, "image")
}

func audioCondition(path string) bool {
	extension := filepath.Ext(path)
	mimetype := mime.TypeByExtension(extension)

	return strings.HasPrefix(mimetype, "audio")
}

func videoCondition(path string) bool {
	extension := filepath.Ext(path)
	mimetype := mime.TypeByExtension(extension)

	return strings.HasPrefix(mimetype, "video")
}

func parseSearch(value string) *searchOptions {
	opts := &searchOptions{
		CaseSensitive: strings.Contains(value, "case:sensitive"),
		Conditions:    []condition{},
		Terms:         []string{},
	}

	// removes the options from the value
	value = strings.ReplaceAll(value, "case:insensitive", "")
	value = strings.ReplaceAll(value, "case:sensitive", "")
	value = strings.TrimSpace(value)

	types := typeRegexp.FindAllStringSubmatch(value, -1)
	for _, t := range types {
		if len(t) == 1 {
			continue
		}

		switch t[1] {
		case "image":
			opts.Conditions = append(opts.Conditions, imageCondition)
		case "audio", "music":
			opts.Conditions = append(opts.Conditions, audioCondition)
		case "video":
			opts.Conditions = append(opts.Conditions, videoCondition)
		default:
			opts.Conditions = append(opts.Conditions, extensionCondition(t[1]))
		}
	}

	if len(types) > 0 {
		// Remove the fields from the search value.
		value = typeRegexp.ReplaceAllString(value, "")
	}

	// If it's case insensitive, the terms are folded as the names are (Gezgin).
	if !opts.CaseSensitive {
		value = fold(value)
	}

	// Remove the spaces from the search value.
	value = strings.TrimSpace(value)

	if value == "" {
		return opts
	}

	// if the value starts with " and finishes what that character, we will
	// only search for that term
	if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
		unique := strings.TrimPrefix(value, "\"")
		unique = strings.TrimSuffix(unique, "\"")

		opts.Terms = []string{unique}
		return opts
	}

	// Every word has to be in the name (Gezgin).
	opts.Terms = strings.Fields(value)
	return opts
}

// fold makes a name or a search term comparable regardless of letter case and
// accents (Gezgin): "IŞIK", "Işık" and "isik" all become "isik". The dotless ı
// has no accent to drop and is read as i.
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		r = unicode.ToLower(r)
		if r == 'ı' {
			r = 'i'
		}
		b.WriteRune(r)
	}
	return b.String()
}
