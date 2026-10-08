package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// Checker is a Rules checker.
type Checker interface {
	Check(path string) bool
}

// Rule is a allow/disallow rule.
type Rule struct {
	Regex  bool    `json:"regex"`
	Allow  bool    `json:"allow"`
	Path   string  `json:"path"`
	Regexp *Regexp `json:"regexp"`
}

// Validate checks rules before they are saved: a path rule needs a path and a regex rule an
// expression that compiles. A broken expression would otherwise fail every request it is matched
// in, and an empty path matches every path.
func Validate(rs []Rule) error {
	for i, r := range rs {
		switch {
		case r.Regex && (r.Regexp == nil || r.Regexp.Raw == ""):
			return fmt.Errorf("%w: rule %d: the expression is empty", fberrors.ErrInvalidRule, i+1)
		case r.Regex:
			if _, err := regexp.Compile(r.Regexp.Raw); err != nil {
				return fmt.Errorf("%w: rule %d: %v", fberrors.ErrInvalidRule, i+1, err)
			}
		case r.Path == "":
			return fmt.Errorf("%w: rule %d: the path is empty", fberrors.ErrInvalidRule, i+1)
		}
	}
	return nil
}

// MatchHidden matches paths with a basename
// that begins with a dot.
func MatchHidden(path string) bool {
	return path != "" && strings.HasPrefix(filepath.Base(path), ".")
}

// Matches matches a path against a rule. When fold is true the comparison is
// case-insensitive: on a case-insensitive filesystem two paths differing only
// in case name the same file, so a rule written for one spelling has to cover
// the others or it can be trivially evaded.
//
// Regex rules are never folded. An admin-authored pattern means what it says,
// and lowering its input would silently change it; use (?i) for those.
func (r *Rule) Matches(path string, fold bool) bool {
	if r.Regex {
		return r.Regexp.MatchString(path)
	}

	rulePath := r.Path
	if fold {
		path = strings.ToLower(path)
		rulePath = strings.ToLower(rulePath)
	}

	if path == rulePath {
		return true
	}

	prefix := rulePath
	if prefix != "/" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	return strings.HasPrefix(path, prefix)
}

// Regexp is a wrapper to the native regexp type where we
// save the raw expression.
type Regexp struct {
	Raw    string `json:"raw"`
	regexp *regexp.Regexp
}

// MatchString checks if a string matches the regexp.
func (r *Regexp) MatchString(s string) bool {
	if r.regexp == nil {
		r.regexp = regexp.MustCompile(r.Raw)
	}

	return r.regexp.MatchString(s)
}
