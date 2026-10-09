package unpack

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// file is a file or folder to put in a test archive.
type file struct {
	name string
	data string
	mode fs.FileMode // zero: a regular file
	// declared is the size a RAR header gives, when it is not the data's.
	declared int
}

func zipOf(t *testing.T, files ...file) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate}
		if f.mode != 0 {
			h.SetMode(f.mode)
		}
		out, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write([]byte(f.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func tarGzOf(t *testing.T, headers ...*tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	w := tar.NewWriter(gz)
	for _, h := range headers {
		data := h.Linkname
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(data))
		} else {
			data = ""
		}
		if h.Mode == 0 && h.Typeflag != tar.TypeXGlobalHeader {
			h.Mode = 0o644
		}
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// tarFile is a regular file for tarGzOf; its content goes in Linkname, which it then clears.
func tarFile(name, data string) *tar.Header {
	return &tar.Header{Name: name, Typeflag: tar.TypeReg, Linkname: data}
}

// rarBlock is a RAR 3 header block: its CRC, type, flags and size, then payload.
func rarBlock(kind byte, flags uint16, payload []byte) []byte {
	body := []byte{kind, byte(flags), byte(flags >> 8), 0, 0}
	binary.LittleEndian.PutUint16(body[3:], uint16(7+len(payload)))
	body = append(body, payload...)
	crc := make([]byte, 2)
	binary.LittleEndian.PutUint16(crc, uint16(crc32.ChecksumIEEE(body)))
	return append(crc, body...)
}

// rarSet makes a RAR 3 set holding files uncompressed, as WinRAR's "store" method does, in
// volumes of at most size bytes of content each (0: one volume). It is enough for sets, names
// and links without a RAR program; the decoders themselves are rardecode's to test.
func rarSet(t *testing.T, size int, newNaming bool, files ...file) [][]byte {
	t.Helper()
	const (
		volume, newNames, firstVolume = 0x0001, 0x0010, 0x0100
		hasData, splitBefore, split   = 0x8000, 0x0001, 0x0002
		dirFlags, notLast             = 0x00e0, 0x0001
	)
	mainFlags := uint16(0)
	if size > 0 {
		mainFlags = volume
		if newNaming {
			mainFlags |= newNames
		}
	}
	var volumes [][]byte
	var cur []byte
	room := size
	start := func() {
		flags := mainFlags
		if len(volumes) == 0 && size > 0 {
			flags |= firstVolume
		}
		cur = append([]byte("Rar!\x1a\x07\x00"), rarBlock(0x73, flags, make([]byte, 6))...)
		room = size
	}
	header := func(f file, flags uint16, part []byte) []byte {
		mode := uint32(0o100644)
		if f.mode&fs.ModeDir != 0 {
			mode = 0o40755
			flags |= dirFlags
		} else if f.mode&fs.ModeSymlink != 0 {
			mode = 0o120777
		}
		size := len(f.data)
		if f.declared > 0 {
			size = f.declared
		}
		p := binary.LittleEndian.AppendUint32(nil, uint32(len(part)))
		p = binary.LittleEndian.AppendUint32(p, uint32(size))
		p = append(p, 3) // made on Unix
		p = binary.LittleEndian.AppendUint32(p, crc32.ChecksumIEEE([]byte(f.data)))
		p = binary.LittleEndian.AppendUint32(p, 0x50210000)
		p = append(p, 20, 0x30) // version 2.0, stored
		p = binary.LittleEndian.AppendUint16(p, uint16(len(f.name)))
		p = binary.LittleEndian.AppendUint32(p, mode)
		p = append(p, f.name...)
		return append(rarBlock(0x74, hasData|flags, p), part...)
	}
	start()
	for _, f := range files {
		data := []byte(f.data)
		flags := uint16(0)
		for {
			if size == 0 || len(data) <= room {
				cur = append(cur, header(f, flags, data)...)
				room -= len(data)
				break
			}
			cur = append(cur, header(f, flags|split, data[:room])...)
			data = data[room:]
			cur = append(cur, rarBlock(0x7b, notLast, nil)...)
			volumes = append(volumes, cur)
			start()
			flags = splitBefore
		}
	}
	cur = append(cur, rarBlock(0x7b, 0, nil)...)
	return append(volumes, cur)
}

// rarNames names the volumes of a set the old way (name.rar, name.r00, ...) or the new way.
func rarNames(stem string, n int, newNaming bool) []string {
	names := make([]string, n)
	for i := range names {
		switch {
		case n > 1 && newNaming:
			names[i] = fmt.Sprintf("%s.part%d.rar", stem, i+1)
		case i == 0:
			names[i] = stem + ".rar"
		default:
			names[i] = fmt.Sprintf("%s.r%02d", stem, i-1)
		}
	}
	return names
}

func sevenZip(t *testing.T, name string) []byte {
	t.Helper()
	// Made with py7zr from belge.txt and klasor/ic.txt; the encrypted ones with "gizli-parola".
	fixtures := map[string]string{
		"duz":             "N3q8ryccAASop0ZJmwAAAAAAAAAVAAAAAAAAAAsmcxMBABs3eiBpY2luZGVuIG1lcmhhYmEKaWMgZG9zeWEKAOAAiABzXQAAgTMHrg/QDrA8nzkQnHGULWw0Ilr6nmib/n4nTMgbGygRvhWrtiX8CcB+ffVhpezZu8DFDZRJBSJ/vul/EoAs+e8uHKYF44arSx/bYV/vkuNiAMAG/nhMbe6um3yObucxqr/RtQVq5LJH8XFyTrYAAAAAABcGIAEJewAHCwEAASEhARgMgIkAAA==",
		"parolali":        "N3q8ryccAAQRccvcswAAAAAAAAAWAAAAAAAAACiZHEydT+kUG3KdGrsEyN6F6L8obKoiIHlomozDKQL9KJsrguAAnACLXQAAgTMHrg/QDv4UYPP1fi0hQVztibum0nUv7ZHOJREdc82263cOLGg4nuqxXRnLifXpgmQeYJiBCTyoT7pfv+n5Nzl3lqZMLnpc6/qL3AgXnUuAZEJzxRZX9kQLGys88+2jyc6Wr2dfoSVEmjFFaMMa5eFyXZewXKwC8yzo8BeaLlquwmy6eIQAAAAAABcGIAEJgJMABwsBAAEhIQEYDICdAAA=",
		"basligi-sifreli": "N3q8ryccAASHZpLXwAAAAAAAAAAqAAAAAAAAAG3I9LMyS8Z5LFmjv42iLvaN6IRML/UbvHRa455iVvOTYTllHmzqry2+7OvIPWMx2Qo/OciBktJeYv8B5+cNqkwDnmM38fJLjcr/Ru4TOE7wArutQZORo3Oy5G19thnEWgH8gAUdM+qMrgxEnzySbh6fLh17IfopvA0TAKbGxrU9tSQlm+D3ArRGN8Wz0tzJx7pMCyqpDOk8HTRsemgLimbWFNIGCEroBBKouLRLEdJGxWY45G/kfLqQo0z9BS/MNVJ08PAXBiABCYCgAAcLAQABJAbxBwESUw//xm9q3urLpewr/6puXyHvDICdAAA=",
	}
	data, err := base64.StdEncoding.DecodeString(fixtures[name])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// env is a source folder and a place for results.
type env struct {
	t   *testing.T
	src string
	out string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	e := &env{t: t, src: filepath.Join(dir, "src"), out: filepath.Join(dir, "out")}
	if err := os.Mkdir(e.src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(e.out, 0o755); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) put(name string, data []byte) {
	e.t.Helper()
	if err := os.WriteFile(filepath.Join(e.src, name), data, 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) putSet(stem string, newNaming bool, volumes [][]byte) []string {
	e.t.Helper()
	names := rarNames(stem, len(volumes), newNaming)
	for i, v := range volumes {
		e.put(names[i], v)
	}
	return names
}

func defaults() Options {
	return Options{FileMode: 0o640, DirMode: 0o750, Limits: Limits{Entries: 10000, Layers: 5, Bytes: 1 << 30}}
}

// extract opens names into out/result and returns what is there, or the reason it stopped.
func (e *env) extract(opt Options, names ...string) (map[string]string, Progress, error) {
	e.t.Helper()
	dst := filepath.Join(e.out, "result")
	_ = os.RemoveAll(dst)
	p, err := Extract(context.Background(), os.DirFS(e.src), names, dst, opt)
	if err != nil {
		return nil, p, err
	}
	return e.tree(dst), p, nil
}

// tree lists a folder: files with their content, folders with a slash.
func (e *env) tree(dir string) map[string]string {
	e.t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(p)
		out[rel] = string(data)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

func want(t *testing.T, got map[string]string, entries ...string) {
	t.Helper()
	var keys []string
	for k, v := range got {
		if strings.HasSuffix(k, "/") {
			keys = append(keys, k)
		} else {
			keys = append(keys, k+"="+v)
		}
	}
	sort.Strings(keys)
	sort.Strings(entries)
	if strings.Join(keys, "\n") != strings.Join(entries, "\n") {
		t.Errorf("result:\n%s\nwant:\n%s", strings.Join(keys, "\n"), strings.Join(entries, "\n"))
	}
}

func TestNamesAndStems(t *testing.T) {
	for name, stem := range map[string]string{
		"film.zip": "film", "Film.ZIP": "Film", "set.part01.rar": "set", "set.part7.rar": "set",
		"rg-42386.rar": "rg-42386", "rg-42386.r15": "rg-42386", "kod.tar.gz": "kod", "kod.tgz": "kod",
		"yedek.tar.zst": "yedek", "x.7z": "x", ".zip": "arsiv", ".gezgin-x.zip": "arsiv",
		"set.part1of3.rar": "set", "set.Part02OF03.rar": "set",
	} {
		if !IsArchive(name) {
			t.Errorf("IsArchive(%q) = false", name)
		}
		if got := Stem(name); got != stem {
			t.Errorf("Stem(%q) = %q; want %q", name, got, stem)
		}
	}
	// Later parts of the largest RAR sets are found from their first part, not chosen; .z01 is a
	// part of a split ZIP.
	for _, name := range []string{"film.mkv", "notlar.txt", "x.gz", "rapor.pdf", "x.r1", "a.s01", "fw.s19", "oyun.z64", "x.z01"} {
		if IsArchive(name) {
			t.Errorf("IsArchive(%q) = true", name)
		}
	}
}

func TestZipWithAFolderAndARarInside(t *testing.T) {
	e := newEnv(t)
	inner := rarSet(t, 0, false, file{name: "ic/c.txt", data: "C"})[0]
	e.put("paket.zip", zipOf(t,
		file{name: "a.txt", data: "A"},
		file{name: "alt/", mode: fs.ModeDir | 0o755},
		file{name: "alt/b.txt", data: "B"},
		file{name: "alt/ic.rar", data: string(inner)},
	))
	got, p, err := e.extract(defaults(), "paket.zip")
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, "a.txt=A", "alt/", "alt/b.txt=B", "alt/ic.rar="+string(inner), "alt/ic/", "alt/ic/ic/", "alt/ic/ic/c.txt=C")
	if p.Archives != 2 || p.Bytes != int64(3+len(inner)) {
		t.Errorf("progress %+v", p)
	}
}

func TestRarSetsOldAndNewNaming(t *testing.T) {
	content := strings.Repeat("0123456789", 40)
	for _, newNaming := range []bool{false, true} {
		e := newEnv(t)
		volumes := rarSet(t, 150, newNaming, file{name: "iso/disk.iso", data: content}, file{name: "oku.nfo", data: "nfo"})
		names := e.putSet("rg", newNaming, volumes)
		if len(volumes) < 3 {
			t.Fatalf("%d volumes", len(volumes))
		}
		// Any part names the set, and two parts of it open it once.
		for _, chosen := range [][]string{{names[0]}, {names[2]}, {names[1], names[0]}} {
			got, p, err := e.extract(defaults(), chosen...)
			if err != nil {
				t.Fatalf("new naming %v, chosen %v: %v", newNaming, chosen, err)
			}
			want(t, got, "iso/", "iso/disk.iso="+content, "oku.nfo=nfo")
			if p.Archives != 1 {
				t.Errorf("%d archives opened", p.Archives)
			}
		}
	}
}

func TestZipsThatEachHoldAPartOfARarSet(t *testing.T) {
	// Chosen together, the ZIPs open into one folder, where the set they hold together opens.
	e := newEnv(t)
	content := strings.Repeat("0123456789", 40)
	volumes := rarSet(t, 150, false, file{name: "film.mkv", data: content})
	parts := rarNames("film", len(volumes), false)
	var zips []string
	for i, v := range volumes {
		name := fmt.Sprintf("film%d.zip", i+1)
		e.put(name, zipOf(t, file{name: parts[i], data: string(v)}, file{name: fmt.Sprintf("file_id%d.diz", i+1), data: "diz"}))
		zips = append(zips, name)
	}
	got, p, err := e.extract(defaults(), zips...)
	if err != nil {
		t.Fatal(err)
	}
	if got["film/film.mkv"] != content || p.Archives != len(zips)+1 {
		t.Errorf("result %v, progress %+v", got, p)
	}
}

func TestRarSetWithAMissingPart(t *testing.T) {
	content := strings.Repeat("x", 500)
	for name, remove := range map[string]int{"first": 0, "middle": 1, "last": -1} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			names := e.putSet("set", false, rarSet(t, 150, false, file{name: "big.bin", data: content}))
			if remove < 0 {
				remove = len(names) - 1
			}
			if err := os.Remove(filepath.Join(e.src, names[remove])); err != nil {
				t.Fatal(err)
			}
			chosen := names[0]
			if remove == 0 {
				chosen = names[1]
			}
			if _, _, err := e.extract(defaults(), chosen); codeOf(err) != CodeMissingPart {
				t.Errorf("err = %v; want missingPart", err)
			}
		})
	}

	// The part that would follow the last one is there: the set is taken as cut short. A part
	// further on is some other file.
	e := newEnv(t)
	names := e.putSet("set", false, rarSet(t, 150, false, file{name: "big.bin", data: content}))
	e.put("set.r40", []byte("not this set's"))
	if got, _, err := e.extract(defaults(), "set.rar"); err != nil || got["big.bin"] != content {
		t.Errorf("with set.r40: err = %v", err)
	}
	e.put(rarNames("set", len(names)+1, false)[len(names)], []byte("next"))
	if _, _, err := e.extract(defaults(), "set.rar"); codeOf(err) != CodeMissingPart {
		t.Errorf("err = %v; want missingPart", err)
	}
}

func TestRarSetNamedPartNofM(t *testing.T) {
	e := newEnv(t)
	content := strings.Repeat("0123456789", 40)
	volumes := rarSet(t, 150, true, file{name: "film.mkv", data: content})
	for i, v := range volumes {
		e.put(fmt.Sprintf("film.part%dof%d.rar", i+1, len(volumes)), v)
	}
	got, _, err := e.extract(defaults(), fmt.Sprintf("film.part2of%d.rar", len(volumes)))
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, "film.mkv="+content)
}

func TestFilesNamedLikeRarPartsStayFiles(t *testing.T) {
	// Firmware, a ROM, a part of a split ZIP, and parts whose set has no first part here: no
	// archive opens, and nothing fails.
	e := newEnv(t)
	e.put("paket.zip", zipOf(t,
		file{name: "fw.s19", data: "S0030000FC"},
		file{name: "oyun.z64", data: "rom"},
		file{name: "yedek.z01", data: "PK\x07\x08"},
		file{name: "eski.r05", data: "Rar!\x1a\x07\x00"},
		file{name: "kayip.part2.rar", data: "Rar!\x1a\x07\x00"},
	))
	got, p, err := e.extract(defaults(), "paket.zip")
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, "fw.s19=S0030000FC", "oyun.z64=rom", "yedek.z01=PK\x07\x08", "eski.r05=Rar!\x1a\x07\x00",
		"kayip.part2.rar=Rar!\x1a\x07\x00")
	if p.Archives != 1 {
		t.Errorf("%d archives opened", p.Archives)
	}

	// Beside a game's RAR, its ROM is no part of it.
	e = newEnv(t)
	e.put("oyun.rar", rarSet(t, 0, false, file{name: "oku.txt", data: "R"})[0])
	e.put("oyun.z64", []byte("rom"))
	if got, _, err := e.extract(defaults(), "oyun.rar"); err != nil || got["oku.txt"] != "R" {
		t.Errorf("oyun.rar: %v, %v", got, err)
	}
	for _, name := range []string{"oyun.z64", "fw.s19"} {
		e.put(name, []byte("x"))
		if _, _, err := e.extract(defaults(), name); codeOf(err) != CodeNotArchive {
			t.Errorf("%s: %v; want notArchive", name, err)
		}
	}
}

func TestRarFileThatEndsShortIsCorrupt(t *testing.T) {
	// Its checksum is that of what is there; only its length says it is cut short.
	e := newEnv(t)
	e.put("kisa.rar", rarSet(t, 0, false, file{name: "a.bin", data: "abc", declared: 13})[0])
	if _, _, err := e.extract(defaults(), "kisa.rar"); codeOf(err) != CodeCorrupt {
		t.Errorf("err = %v; want corrupt", err)
	}
}

func TestUnsafeEntriesAreRefused(t *testing.T) {
	cases := map[string][]byte{
		"parent":     zipOf(t, file{name: "../kacak.txt", data: "x"}),
		"absolute":   zipOf(t, file{name: "/etc/kacak", data: "x"}),
		"deep":       zipOf(t, file{name: "a/../../kacak.txt", data: "x"}),
		"reserved":   zipOf(t, file{name: "klasor/.gezgin-cop/x", data: "x"}),
		"control":    zipOf(t, file{name: "satir\nsonu.txt", data: "x"}),
		"twice":      zipOf(t, file{name: "a.txt", data: "1"}, file{name: "a.txt", data: "2"}),
		"fileAndDir": zipOf(t, file{name: "a", data: "1"}, file{name: "a/b", data: "2"}),
		"zipLink":    zipOf(t, file{name: "bag", data: "/etc/passwd", mode: fs.ModeSymlink | 0o777}),
		"rarLink":    rarSet(t, 0, false, file{name: "bag", data: "/etc/passwd", mode: fs.ModeSymlink})[0],
		"tarLink":    tarGzOf(t, &tar.Header{Name: "bag", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}),
		"tarHard":    tarGzOf(t, tarFile("a", "A"), &tar.Header{Name: "b", Typeflag: tar.TypeLink, Linkname: "a"}),
		"tarDevice":  tarGzOf(t, &tar.Header{Name: "aygit", Typeflag: tar.TypeChar}),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			archive := name + ".zip"
			switch {
			case strings.HasPrefix(name, "rar"):
				archive = name + ".rar"
			case strings.HasPrefix(name, "tar"):
				archive = name + ".tar.gz"
			}
			e.put(archive, data)
			if _, _, err := e.extract(defaults(), archive); codeOf(err) != CodeUnsafe {
				t.Errorf("err = %v; want unsafe", err)
			}
			// Nothing was written beside the result.
			entries, _ := os.ReadDir(e.out)
			if len(entries) != 1 || entries[0].Name() != "result" {
				t.Errorf("out holds %v", entries)
			}
		})
	}
}

func TestTarDotFoldersAndModes(t *testing.T) {
	e := newEnv(t)
	e.put("kod.tgz", tarGzOf(t,
		&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o777},
		&tar.Header{Name: "./src/", Typeflag: tar.TypeDir, Mode: 0o777},
		tarFile("./src/main.go", "package main"),
		&tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "git"}},
	))
	opt := defaults()
	got, _, err := e.extract(opt, "kod.tgz")
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, "src/", "src/main.go=package main")
	for name, mode := range map[string]fs.FileMode{"": opt.DirMode, "src": opt.DirMode, "src/main.go": opt.FileMode} {
		info, err := os.Stat(filepath.Join(e.out, "result", name))
		if err != nil || info.Mode().Perm() != mode {
			t.Errorf("%q: %v %v; want mode %v", name, info.Mode(), err, mode)
		}
	}
}

func TestLimits(t *testing.T) {
	var many []file
	for i := range 30 {
		many = append(many, file{name: fmt.Sprintf("f%02d.txt", i), data: "x"})
	}
	var manyTar []*tar.Header
	for i := range 30 {
		manyTar = append(manyTar, tarFile(fmt.Sprintf("f%02d.txt", i), "x"))
	}
	nested := zipOf(t, file{name: "a.txt", data: "a"})
	for range 3 {
		nested = zipOf(t, file{name: "ic.zip", data: string(nested)})
	}

	e := newEnv(t)
	e.put("cok.zip", zipOf(t, many...))
	e.put("cok.tar.gz", tarGzOf(t, manyTar...))
	e.put("buyuk.zip", zipOf(t, file{name: "buyuk.bin", data: strings.Repeat("0", 100000)}))
	e.put("ic-ice.zip", nested)

	few := defaults()
	few.Limits.Entries = 10
	small := defaults()
	small.Limits.Bytes = 1000
	shallow := defaults()
	shallow.Limits.Layers = 3

	for _, c := range []struct {
		name string
		opt  Options
		code Code
	}{
		{"cok.zip", few, CodeEntries},
		{"cok.tar.gz", few, CodeEntries},
		{"buyuk.zip", small, CodeNoSpace},
		{"ic-ice.zip", shallow, CodeLayers},
	} {
		if _, _, err := e.extract(c.opt, c.name); codeOf(err) != c.code {
			t.Errorf("%s: err = %v; want %s", c.name, err, c.code)
		}
	}

	got, p, err := e.extract(defaults(), "ic-ice.zip")
	if err != nil || got["ic/ic/ic/a.txt"] != "a" || p.Archives != 4 {
		t.Errorf("four layers: %v %v %+v", got, err, p)
	}
}

func TestSevenZipAndPasswords(t *testing.T) {
	e := newEnv(t)
	e.put("duz.7z", sevenZip(t, "duz"))
	e.put("parolali.7z", sevenZip(t, "parolali"))
	e.put("basligi-sifreli.7z", sevenZip(t, "basligi-sifreli"))
	contents := []string{"belge.txt=7z icinden merhaba\n", "klasor/", "klasor/ic.txt=ic dosya\n"}

	got, _, err := e.extract(defaults(), "duz.7z")
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, contents...)

	for _, name := range []string{"parolali.7z", "basligi-sifreli.7z"} {
		if _, _, err = e.extract(defaults(), name); codeOf(err) != CodePassword {
			t.Errorf("%s without a password: %v; want password", name, err)
		}
		wrong := defaults()
		wrong.Password = "yanlis-parola"
		if _, _, err = e.extract(wrong, name); codeOf(err) != CodeWrongPassword {
			t.Errorf("%s with a wrong password: %v; want wrongPassword", name, err)
		}
		right := defaults()
		right.Password = "gizli-parola"
		got, _, err = e.extract(right, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want(t, got, contents...)
	}
}

func TestZipNamesAndEncryption(t *testing.T) {
	e := newEnv(t)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// Windows wrote "ışık.txt" in code page 857, and a folder with a backslash.
	for _, name := range []string{"\x8d\x9f\x8dk.txt", `klasor\dosya.txt`} {
		out, err := w.CreateHeader(&zip.FileHeader{Name: name, NonUTF8: true})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = out.Write([]byte("x"))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	e.put("windows.zip", buf.Bytes())
	got, _, err := e.extract(defaults(), "windows.zip")
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, "ışık.txt=x", "klasor/", "klasor/dosya.txt=x")

	buf.Reset()
	w = zip.NewWriter(&buf)
	out, err := w.CreateRaw(&zip.FileHeader{Name: "gizli.txt", Flags: 0x1, Method: zip.Store,
		CRC32: crc32.ChecksumIEEE([]byte("x")), CompressedSize64: 1, UncompressedSize64: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = out.Write([]byte("x"))
	_ = w.Close()
	e.put("sifreli.zip", buf.Bytes())
	if _, _, err = e.extract(defaults(), "sifreli.zip"); codeOf(err) != CodeUnsupported {
		t.Errorf("an encrypted ZIP: %v; want unsupported", err)
	}
}

func TestNotArchives(t *testing.T) {
	e := newEnv(t)
	e.put("sahte.zip", []byte("this is not a zip"))
	e.put("notlar.txt", []byte("text"))
	for _, name := range []string{"sahte.zip", "notlar.txt"} {
		if _, _, err := e.extract(defaults(), name); codeOf(err) != CodeNotArchive {
			t.Errorf("%s: %v; want notArchive", name, err)
		}
	}

	// Inside an archive, a file named like one that is not stays as it is.
	e.put("paket.zip", zipOf(t, file{name: "sahte.zip", data: "not a zip"}, file{name: "a.txt", data: "A"}))
	got, p, err := e.extract(defaults(), "paket.zip")
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, "sahte.zip=not a zip", "a.txt=A")
	if p.Archives != 1 {
		t.Errorf("%d archives opened", p.Archives)
	}
}

func TestInnerFoldersTakeFreeNames(t *testing.T) {
	e := newEnv(t)
	inner := zipOf(t, file{name: "c.txt", data: "C"})
	e.put("paket.zip", zipOf(t, file{name: "ic/var.txt", data: "V"}, file{name: "ic.zip", data: string(inner)}))
	got, _, err := e.extract(defaults(), "paket.zip")
	if err != nil {
		t.Fatal(err)
	}
	want(t, got, "ic/", "ic/var.txt=V", "ic.zip="+string(inner), "ic (2)/", "ic (2)/c.txt=C")
}

// changingFS reports a newer time for every file it is asked about by name.
type changingFS struct{ fstest.MapFS }

func (c changingFS) Stat(name string) (fs.FileInfo, error) {
	f := *c.MapFS[name]
	f.ModTime = f.ModTime.Add(time.Second)
	return fstest.MapFS{name: &f}.Stat(name)
}

func TestASourceThatChangesIsRefused(t *testing.T) {
	src := changingFS{fstest.MapFS{"a.zip": {Data: zipOf(t, file{name: "a.txt", data: "A"}), ModTime: time.Now()}}}
	_, err := Extract(context.Background(), src, []string{"a.zip"}, filepath.Join(t.TempDir(), "result"), defaults())
	if codeOf(err) != CodeChanged {
		t.Errorf("err = %v; want changed", err)
	}
}

func TestCancel(t *testing.T) {
	e := newEnv(t)
	e.put("a.zip", zipOf(t, file{name: "a.txt", data: "A"}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Extract(ctx, os.DirFS(e.src), []string{"a.zip"}, filepath.Join(e.out, "result"), defaults())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v; want canceled", err)
	}
}
