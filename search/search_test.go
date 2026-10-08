package search

import (
	"context"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// refuse is a rule checker that refuses a folder and remembers what it was asked about.
type refuse struct {
	folder string
	asked  []string
}

func (r *refuse) Check(p string) bool {
	r.asked = append(r.asked, p)
	return p != r.folder && !strings.HasPrefix(p, r.folder+"/")
}

func searchTree(t *testing.T) afero.Fs {
	t.Helper()
	fs := afero.NewMemMapFs()
	for _, name := range []string{
		"/alfa beta.txt", "/alfa.txt", "/beta.txt", "/İstanbul.txt", "/IŞIK.txt", "/Işık notları.txt",
		"/RAPOR.PDF", "/photo.JPG", "/gizli/kapali.txt", "/acik/kapali notlar.txt",
	} {
		if err := afero.WriteFile(fs, name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return fs
}

func run(t *testing.T, fs afero.Fs, query string, checker *refuse) []string {
	t.Helper()
	found := []string{}
	err := Search(context.Background(), fs, "/", query, checker, func(p string, _ os.FileInfo) error {
		found = append(found, p)
		return nil
	})
	if err != nil {
		t.Fatalf("search %q: %v", query, err)
	}
	sort.Strings(found)
	return found
}

func TestSearchMatchesEveryWordRegardlessOfCaseAndAccents(t *testing.T) {
	fs := searchTree(t)
	for query, want := range map[string][]string{
		"alfa beta":            {"alfa beta.txt"},
		"alfa  beta":           {"alfa beta.txt"},
		"beta alfa":            {"alfa beta.txt"},
		`"alfa beta"`:          {"alfa beta.txt"},
		"ışık":                 {"IŞIK.txt", "Işık notları.txt"},
		"isik":                 {"IŞIK.txt", "Işık notları.txt"},
		"IŞIK":                 {"IŞIK.txt", "Işık notları.txt"},
		"işık notlari":         {"Işık notları.txt"},
		"istanbul":             {"İstanbul.txt"},
		"İSTANBUL":             {"İstanbul.txt"},
		"case:sensitive IŞIK":  {"IŞIK.txt"},
		"case:sensitive ışık":  {},
		"type:pdf":             {"RAPOR.PDF"},
		"type:image":           {"photo.JPG"},
		"type:image photo":     {"photo.JPG"},
		"kapali":               {"acik/kapali notlar.txt", "gizli/kapali.txt"},
		"nothing matches this": {},
	} {
		if got := run(t, fs, query, &refuse{folder: "/none"}); !reflect.DeepEqual(got, want) {
			t.Errorf("search %q = %v, want %v", query, got, want)
		}
	}
}

func TestSearchDoesNotEnterRefusedFolders(t *testing.T) {
	fs := searchTree(t)
	checker := &refuse{folder: "/gizli"}
	if got, want := run(t, fs, "kapali", checker), []string{"acik/kapali notlar.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("search = %v, want %v", got, want)
	}
	for _, p := range checker.asked {
		if strings.HasPrefix(p, "/gizli/") {
			t.Fatalf("the search walked into the refused folder: %s", p)
		}
	}
}
