package files

import (
	"testing"

	"github.com/spf13/afero"
)

// K82: a listing reads no header, so archives and their parts small enough, and files whose
// extension names a type that is not text, passed for text. They are archives and files now;
// opening one still reads its header.
func TestListingTypes(t *testing.T) {
	memFs := afero.NewMemMapFs()
	want := map[string]string{
		"kucuk.zip":         "archive",
		"film.r00":          "archive",
		"film.part2.rar":    "archive",
		"film.zip.001":      "archive",
		"kod.tgz":           "archive",
		"modul.wasm":        "blob",
		"veri.json":         "text",
		"OKU.md":            "text",
		"Makefile":          "text",
		"firmware.s19":      "text",
		"resim.png":         "image",
		"bolum.zip.001.txt": "text",
	}
	for name := range want {
		if err := afero.WriteFile(memFs, "/"+name, []byte("PK\x03\x04 kucuk"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dir, err := NewFileInfo(&FileOptions{Fs: memFs, Path: "/", Expand: true, Checker: allowAllChecker{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range dir.Items {
		if item.Type != want[item.Name] {
			t.Errorf("%s listed as %q; want %q", item.Name, item.Type, want[item.Name])
		}
	}

	// Opened, a small ZIP is a binary file, as before.
	file, err := NewFileInfo(&FileOptions{Fs: memFs, Path: "/kucuk.zip", Expand: true, Checker: allowAllChecker{}})
	if err != nil {
		t.Fatal(err)
	}
	if file.Type != "blob" {
		t.Errorf("opened kucuk.zip is %q; want blob", file.Type)
	}
}
