package pack

import (
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"path"
	"strings"
	"time"

	"github.com/klauspost/compress/flate"
)

// zipWriter writes a ZIP archive (PKWARE APPNOTE 6.3). Into files, it fills each file's
// checksum and sizes into its local header once its data is written, so that no entry needs a
// data descriptor; into a stream it writes descriptors. A file of about 4 GiB or more has its
// sizes in a ZIP64 field of the local header too, and the central directory's ZIP64 field
// holds only what does not fit in the header: Go's own writer does neither, and 7-Zip then
// reports a header error.
type zipWriter struct {
	out     *output
	loc     *time.Location
	entries []*zipEntry
	deflate *flate.Writer
	data    zipData
	force64 bool // ZIP64 fields everywhere, for tests
}

type zipEntry struct {
	name     string
	offset   int64 // of its local header
	method   uint16
	flags    uint16
	dir      bool
	exec     bool
	zip64    bool // its sizes are in ZIP64 fields
	modified time.Time
	crc      uint32
	csize    int64
	usize    int64
}

const (
	zipLocalSig      = 0x04034b50
	zipCentralSig    = 0x02014b50
	zipDescriptorSig = 0x08074b50
	zip64EndSig      = 0x06064b50
	zip64LocatorSig  = 0x07064b50
	zipEndSig        = 0x06054b50

	zip64ExtraID   = 0x0001
	zipTimeExtraID = 0x5455 // Info-ZIP's extended timestamp: the time in UTC

	zipStore      = 0
	zipDeflate    = 8
	zipDescriptor = 0x8
	zipUTF8       = 0x800

	zipVersion20 = 20
	zipVersion45 = 45 // ZIP64
	zipUnix      = 3 << 8

	uint16max = 1<<16 - 1
	uint32max = 1<<32 - 1

	// zip64From is the size from which a file's sizes are kept in ZIP64 fields: deflate makes
	// data that does not compress a little larger.
	zip64From = uint32max - 1<<26
)

func newZipWriter(out *output, loc *time.Location) *zipWriter {
	if loc == nil {
		loc = time.Local
	}
	fw, _ := flate.NewWriter(io.Discard, 5) // a valid level does not fail
	return &zipWriter{out: out, loc: loc, deflate: fw, data: zipData{crc: crc32.NewIEEE()}, force64: forceZip64}
}

// forceZip64 makes every ZIP use ZIP64 fields, for tests.
var forceZip64 = false

// zipData writes a file's data, keeping its checksum and size.
type zipData struct {
	w   io.Writer
	crc hash.Hash32
	n   int64
}

func (d *zipData) Write(p []byte) (int, error) {
	d.crc.Write(p)
	n, err := d.w.Write(p)
	d.n += int64(n)
	return n, err
}

// counter counts what is written through it.
type counter struct {
	w io.Writer
	n int64
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func (z *zipWriter) dir(e entry) error {
	_, err := z.begin(e, zipStore)
	return err
}

func (z *zipWriter) file(e entry, data func(io.Writer) error) error {
	method := uint16(zipDeflate)
	if stored(e.name) || e.info.Size() == 0 {
		method = zipStore
	}
	ze, err := z.begin(e, method)
	if err != nil {
		return err
	}

	compressed := &counter{w: z.out}
	z.data.w, z.data.n = compressed, 0
	z.data.crc.Reset()
	if method == zipDeflate {
		z.deflate.Reset(compressed)
		z.data.w = z.deflate
	}
	if err := data(&z.data); err != nil {
		return err
	}
	if method == zipDeflate {
		if err := z.deflate.Close(); err != nil {
			return err
		}
	}
	ze.crc, ze.csize, ze.usize = z.data.crc.Sum32(), compressed.n, z.data.n
	if !ze.zip64 && (ze.csize >= uint32max || ze.usize >= uint32max) {
		return errors.New("pack: a file outgrew its planned size")
	}

	if !z.out.seekable() {
		b := le32(nil, zipDescriptorSig)
		b = le32(b, ze.crc)
		if ze.zip64 {
			b = le64(b, ze.csize)
			b = le64(b, ze.usize)
		} else {
			b = le32(b, uint32(ze.csize))
			b = le32(b, uint32(ze.usize))
		}
		_, err := z.out.Write(b)
		return err
	}
	b := le32(nil, ze.crc)
	if ze.zip64 {
		b = le32(le32(b, uint32max), uint32max)
	} else {
		b = le32(le32(b, uint32(ze.csize)), uint32(ze.usize))
	}
	if err := z.out.writeAt(b, ze.offset+14); err != nil {
		return err
	}
	if ze.zip64 {
		// The ZIP64 field leads the local header's extra fields: its id, its size, then the sizes.
		at := ze.offset + 30 + int64(len(ze.name)) + 4
		if err := z.out.writeAt(le64(le64(nil, ze.usize), ze.csize), at); err != nil {
			return err
		}
	}
	return nil
}

// begin writes an entry's local header, with its checksum and sizes left to be filled in.
func (z *zipWriter) begin(e entry, method uint16) (*zipEntry, error) {
	ze := &zipEntry{
		name:     e.name,
		offset:   z.out.count,
		method:   method,
		dir:      e.info.IsDir(),
		exec:     e.info.Mode()&0o111 != 0,
		modified: e.info.ModTime(),
	}
	ze.zip64 = !ze.dir && (z.force64 || e.info.Size() >= zip64From)
	if !ascii(ze.name) {
		ze.flags |= zipUTF8
	}
	if !ze.dir && !z.out.seekable() {
		ze.flags |= zipDescriptor
	}

	var extra []byte
	if ze.zip64 {
		extra = le16(le16(extra, zip64ExtraID), 16)
		extra = le64(le64(extra, 0), 0)
	}
	extra = timeExtra(extra, ze.modified)

	date, clock := dosTime(ze.modified, z.loc)
	b := le32(nil, zipLocalSig)
	b = le16(b, ze.version())
	b = le16(b, ze.flags)
	b = le16(b, ze.method)
	b = le16(b, clock)
	b = le16(b, date)
	b = le32(b, 0) // checksum
	if ze.zip64 {
		b = le32(le32(b, uint32max), uint32max)
	} else {
		b = le32(le32(b, 0), 0)
	}
	b = le16(b, uint16(len(ze.name)))
	b = le16(b, uint16(len(extra)))
	b = append(b, ze.name...)
	b = append(b, extra...)
	if _, err := z.out.Write(b); err != nil {
		return nil, err
	}
	z.entries = append(z.entries, ze)
	return ze, nil
}

func (ze *zipEntry) version() uint16 {
	if ze.zip64 {
		return zipVersion45
	}
	return zipVersion20
}

// close writes the central directory and the end records.
func (z *zipWriter) close() error {
	start := z.out.count
	for _, ze := range z.entries {
		if _, err := z.out.Write(z.central(ze)); err != nil {
			return err
		}
	}
	size, count := z.out.count-start, int64(len(z.entries))

	var b []byte
	if z.force64 || count >= uint16max || size >= uint32max || start >= uint32max {
		at := z.out.count
		b = le32(b, zip64EndSig)
		b = le64(b, 44) // the size of the rest of the record
		b = le16(b, zipUnix|zipVersion45)
		b = le16(b, zipVersion45)
		b = le32(le32(b, 0), 0) // this disk, the directory's disk
		b = le64(le64(b, count), count)
		b = le64(le64(b, size), start)
		b = le32(b, zip64LocatorSig)
		b = le32(b, 0)
		b = le64(b, at)
		b = le32(b, 1) // disks
	}
	b = le32(b, zipEndSig)
	b = le32(b, 0) // this disk, the directory's disk
	count16 := uint16(min(count, uint16max))
	if z.force64 {
		count16 = uint16max
	}
	b = le16(le16(b, count16), count16)
	b = le32(b, clamp32(size, z.force64))
	b = le32(b, clamp32(start, z.force64))
	b = le16(b, 0) // comment
	_, err := z.out.Write(b)
	return err
}

// central is an entry's central directory header. Its ZIP64 field, when it has one, holds the
// sizes and the offset that do not fit in the header, and nothing else.
func (z *zipWriter) central(ze *zipEntry) []byte {
	far := z.force64 || ze.offset >= uint32max
	var extra []byte
	if ze.zip64 || far {
		n := uint16(0)
		if ze.zip64 {
			n += 16
		}
		if far {
			n += 8
		}
		extra = le16(le16(extra, zip64ExtraID), n)
		if ze.zip64 {
			extra = le64(le64(extra, ze.usize), ze.csize)
		}
		if far {
			extra = le64(extra, ze.offset)
		}
	}
	extra = timeExtra(extra, ze.modified)

	version := ze.version()
	if far {
		version = zipVersion45
	}
	mode, attrs := uint32(0o100644), uint32(0)
	switch {
	case ze.dir:
		mode, attrs = 0o40755, 0x10 // MS-DOS: a folder
	case ze.exec:
		mode = 0o100755
	}

	date, clock := dosTime(ze.modified, z.loc)
	b := le32(nil, zipCentralSig)
	b = le16(b, zipUnix|version)
	b = le16(b, version)
	b = le16(b, ze.flags)
	b = le16(b, ze.method)
	b = le16(b, clock)
	b = le16(b, date)
	b = le32(b, ze.crc)
	if ze.zip64 {
		b = le32(le32(b, uint32max), uint32max)
	} else {
		b = le32(le32(b, uint32(ze.csize)), uint32(ze.usize))
	}
	b = le16(b, uint16(len(ze.name)))
	b = le16(b, uint16(len(extra)))
	b = le16(b, 0)          // comment
	b = le16(le16(b, 0), 0) // disk, internal attributes
	b = le32(b, mode<<16|attrs)
	b = le32(b, clamp32(ze.offset, far))
	b = append(b, ze.name...)
	return append(b, extra...)
}

// clamp32 is v in a 32-bit field, or the mark that sends a reader to the ZIP64 field.
func clamp32(v int64, far bool) uint32 {
	if far || v >= uint32max {
		return uint32max
	}
	return uint32(v)
}

// dosTime is t in MS-DOS form, in loc: Windows shows it as it is. The form holds the years
// 1980 to 2107, in steps of two seconds.
func dosTime(t time.Time, loc *time.Location) (date, clock uint16) {
	t = t.In(loc)
	switch {
	case t.Year() < 1980:
		t = time.Date(1980, 1, 1, 0, 0, 0, 0, loc)
	case t.Year() > 2107:
		t = time.Date(2107, 12, 31, 23, 59, 58, 0, loc)
	}
	date = uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
	clock = uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
	return date, clock
}

// timeExtra adds the extended timestamp, which most readers prefer, for a time it can hold.
func timeExtra(b []byte, t time.Time) []byte {
	unix := t.Unix()
	if unix < 0 || unix > 1<<31-1 {
		return b
	}
	b = le16(le16(b, zipTimeExtraID), 5)
	b = append(b, 1) // the modification time only
	return le32(b, uint32(unix))
}

func ascii(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func le16(b []byte, v uint16) []byte { return binary.LittleEndian.AppendUint16(b, v) }
func le32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }
func le64(b []byte, v int64) []byte  { return binary.LittleEndian.AppendUint64(b, uint64(v)) }

// storedExtensions name files that are compressed already: a ZIP keeps them as they are.
var storedExtensions = map[string]bool{}

func init() {
	for _, ext := range strings.Fields(`
		mkv mp4 m4v avi mov webm ts m2ts mts wmv flv mpg mpeg vob 3gp ogv
		mp3 m4a m4b aac flac ogg oga opus wma
		jpg jpeg png gif webp heic heif avif jxl
		zip zipx rar 7z gz tgz bz2 tbz tbz2 xz txz zst tzst lz4 lz lzma br cab
		iso dmg pdf epub docx xlsx pptx odt ods odp jar apk ipa cbz cbr woff woff2`) {
		storedExtensions["."+ext] = true
	}
}

func stored(name string) bool {
	return storedExtensions[strings.ToLower(path.Ext(name))]
}
