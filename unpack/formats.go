package unpack

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/bodgit/sevenzip"
	"github.com/klauspost/compress/zip"
	"github.com/mholt/archives"
	"github.com/nwaples/rardecode/v2"
)

// known reports whether err already says why a job stopped (a reason, a cancellation or a full
// disk), and returns it so.
func known(err error) (bool, error) {
	var e *Error
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &e), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return true, err
	case errors.Is(err, syscall.ENOSPC):
		return true, fail(CodeNoSpace, err)
	}
	return false, nil
}

func (j *job) rar(fsys *watchFS, u unit) error {
	options := []rardecode.Option{rardecode.FileSystem(fsys), rardecode.MaxDictionarySize(maxDictionary)}
	if j.opt.Password != "" {
		options = append(options, rardecode.Password(j.opt.Password))
	}
	r, err := rardecode.OpenReader(u.file(), options...)
	if err != nil {
		return j.rarError(err, j.opt.Password != "")
	}
	defer r.Close()

	// What fails to read is taken as encrypted once an encrypted header was seen, or for the
	// first header when a password was given: a wrong one makes encrypted headers unreadable.
	encrypted := j.opt.Password != ""
	for {
		if err = j.ctx.Err(); err != nil {
			return err
		}
		h, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return j.rarError(err, encrypted)
		}
		encrypted = h.Encrypted || h.HeaderEncrypted
		err = j.put(u.out, entry{
			name: h.Name,
			dir:  h.IsDir,
			mode: h.Mode(),
			time: h.ModificationTime,
			// Read, not WriteTo: rardecode's WriteTo takes a file that ends short, as a link's
			// does, for a whole one.
			open: func() (io.ReadCloser, error) { return io.NopCloser(struct{ io.Reader }{r}), nil },
		})
		if err != nil {
			return j.rarError(err, encrypted)
		}
	}

	// A set ends where its last part says so. rardecode also ends one whose part has no end
	// record when the next is not there, naming that missing part among those it read; and a
	// set that ends while its next part is there is cut short.
	used := r.Volumes()
	for _, name := range used {
		if _, err := fs.Stat(fsys.FS, path.Join(u.dir, name)); err != nil {
			return fail(CodeMissingPart, err)
		}
	}
	if more, err := continues(fsys.FS, u.dir, used[len(used)-1]); err != nil {
		return err
	} else if more {
		return fail(CodeMissingPart, errors.New("the set ended before its last part"))
	}
	return nil
}

// rarError says why a RAR could not be read; encrypted tells whether what was read was.
func (j *job) rarError(err error, encrypted bool) error {
	if ok, e := known(err); ok {
		return e
	}
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, fs.ErrPermission):
		return fail(CodeMissingPart, err)
	case errors.Is(err, rardecode.ErrArchiveEncrypted), errors.Is(err, rardecode.ErrArchivedFileEncrypted):
		if j.opt.Password == "" {
			return fail(CodePassword, err)
		}
		return fail(CodeWrongPassword, err)
	case errors.Is(err, rardecode.ErrBadPassword):
		return fail(CodeWrongPassword, err)
	case errors.Is(err, rardecode.ErrDictionaryTooLarge), errors.Is(err, rardecode.ErrUnknownDecoder),
		errors.Is(err, rardecode.ErrUnsupportedDecoder), errors.Is(err, rardecode.ErrUnknownEncryptMethod),
		errors.Is(err, rardecode.ErrMultipleDecoders):
		return fail(CodeUnsupported, err)
	case errors.Is(err, rardecode.ErrNoSig):
		return fail(CodeNotArchive, err)
	case encrypted && j.opt.Password != "":
		// An older RAR read with a wrong password only fails its checks.
		return fail(CodeWrongPassword, err)
	}
	return fail(CodeCorrupt, err)
}

// readerAt is an archive file that can be read anywhere, as ZIP and 7z need.
type readerAt interface {
	io.ReaderAt
	io.Seeker
}

func (j *job) zip(fsys *watchFS, u unit) error {
	f, err := fsys.Open(u.file())
	if err != nil {
		return err
	}
	defer f.Close()
	file, ok := f.(readerAt)
	if !ok {
		return errors.New("the file cannot be read at random")
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// The directory is read whole before any entry: its count has to fit first.
	if n, ok := zipCount(file, info.Size()); ok && n > uint64(j.opt.Limits.Entries-j.progress.Entries) {
		return fail(CodeEntries, nil)
	}

	err = archives.Zip{}.Extract(j.ctx, f, func(_ context.Context, fi archives.FileInfo) error {
		if hdr, ok := fi.Header.(zip.FileHeader); ok && hdr.Flags&0x1 != 0 {
			return fail(CodeUnsupported, errors.New("an encrypted ZIP"))
		}
		return j.put(u.out, entry{
			// Windows tools have written folders with backslashes.
			name: strings.ReplaceAll(fi.NameInArchive, `\`, "/"),
			dir:  fi.IsDir(),
			mode: fi.Mode(),
			time: fi.ModTime(),
			open: func() (io.ReadCloser, error) { return fi.Open() },
		})
	})
	if ok, e := known(err); ok {
		return e
	}
	switch {
	case errors.Is(err, zip.ErrFormat):
		return fail(CodeNotArchive, err)
	case errors.Is(err, zip.ErrAlgorithm):
		return fail(CodeUnsupported, err)
	}
	return fail(CodeCorrupt, err)
}

// zipCount reads how many entries a ZIP file's directory has, from its end record.
func zipCount(r io.ReaderAt, size int64) (uint64, bool) {
	const endLen, end64Len, locatorLen = 22, 56, 20
	tail := int64(endLen + 65535)
	if tail > size {
		tail = size
	}
	buf := make([]byte, tail)
	if _, err := r.ReadAt(buf, size-tail); err != nil && !errors.Is(err, io.EOF) {
		return 0, false
	}
	at := bytes.LastIndex(buf, []byte("PK\x05\x06"))
	if at < 0 || len(buf)-at < endLen {
		return 0, false
	}
	count := uint64(binary.LittleEndian.Uint16(buf[at+10:]))
	if count != 0xffff {
		return count, true
	}
	// ZIP64: the locator before the end record tells where the larger record is.
	if at < locatorLen || !bytes.Equal(buf[at-locatorLen:at-locatorLen+4], []byte("PK\x06\x07")) {
		return 0, false
	}
	offset := int64(binary.LittleEndian.Uint64(buf[at-locatorLen+8:]))
	record := make([]byte, end64Len)
	if offset < 0 || offset > size-end64Len {
		return 0, false
	}
	if _, err := r.ReadAt(record, offset); err != nil || !bytes.Equal(record[:4], []byte("PK\x06\x06")) {
		return 0, false
	}
	return binary.LittleEndian.Uint64(record[32:]), true
}

func (j *job) sevenZip(fsys *watchFS, u unit) error {
	f, err := fsys.Open(u.file())
	if err != nil {
		return err
	}
	defer f.Close()
	if _, ok := f.(readerAt); !ok {
		return errors.New("the file cannot be read at random")
	}

	err = archives.SevenZip{Password: j.opt.Password}.Extract(j.ctx, f, func(_ context.Context, fi archives.FileInfo) error {
		return j.put(u.out, entry{
			name: fi.NameInArchive,
			dir:  fi.IsDir(),
			mode: fi.Mode(),
			time: fi.ModTime(),
			open: func() (io.ReadCloser, error) { return fi.Open() },
		})
	})
	if ok, e := known(err); ok {
		return e
	}
	var readErr *sevenzip.ReadError
	if errors.As(err, &readErr) && readErr.Encrypted {
		if j.opt.Password == "" {
			return fail(CodePassword, err)
		}
		return fail(CodeWrongPassword, err)
	}
	return fail(CodeCorrupt, err)
}

func (j *job) tar(fsys *watchFS, u unit) error {
	f, err := fsys.Open(u.file())
	if err != nil {
		return err
	}
	defer f.Close()

	var extractor archives.Extractor = archives.Tar{}
	if u.kind.compression != nil {
		extractor = archives.CompressedArchive{Compression: u.kind.compression, Extraction: archives.Tar{}}
	}
	err = extractor.Extract(j.ctx, f, func(_ context.Context, fi archives.FileInfo) error {
		// A hard link reads as an empty regular file; only files and folders come out.
		if hdr, ok := fi.Header.(*tar.Header); ok {
			switch hdr.Typeflag {
			case tar.TypeReg, tar.TypeRegA, tar.TypeDir: //nolint:staticcheck // old tars still use TypeRegA
			default:
				return fail(CodeUnsafe, errors.New("a link or a special file"))
			}
		}
		return j.put(u.out, entry{
			name: fi.NameInArchive,
			dir:  fi.IsDir(),
			mode: fi.Mode(),
			time: fi.ModTime(),
			open: func() (io.ReadCloser, error) { return fi.Open() },
		})
	})
	if ok, e := known(err); ok {
		return e
	}
	return fail(CodeCorrupt, err)
}

// utf8Name returns an entry's name in UTF-8. Archives made on Windows without Unicode names
// keep them in the OEM code page; for Gezgin's users that is the Turkish one, 857.
func utf8Name(name string) string {
	if utf8.ValidString(name) {
		return name
	}
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(cp857[c-0x80])
		}
	}
	return b.String()
}

// cp857 holds the characters of code page 857 above ASCII.
var cp857 = [128]rune{
	0x00C7, 0x00FC, 0x00E9, 0x00E2, 0x00E4, 0x00E0, 0x00E5, 0x00E7,
	0x00EA, 0x00EB, 0x00E8, 0x00EF, 0x00EE, 0x0131, 0x00C4, 0x00C5,
	0x00C9, 0x00E6, 0x00C6, 0x00F4, 0x00F6, 0x00F2, 0x00FB, 0x00F9,
	0x0130, 0x00D6, 0x00DC, 0x00F8, 0x00A3, 0x00D8, 0x015E, 0x015F,
	0x00E1, 0x00ED, 0x00F3, 0x00FA, 0x00F1, 0x00D1, 0x011E, 0x011F,
	0x00BF, 0x00AE, 0x00AC, 0x00BD, 0x00BC, 0x00A1, 0x00AB, 0x00BB,
	0x2591, 0x2592, 0x2593, 0x2502, 0x2524, 0x00C1, 0x00C2, 0x00C0,
	0x00A9, 0x2563, 0x2551, 0x2557, 0x255D, 0x00A2, 0x00A5, 0x2510,
	0x2514, 0x2534, 0x252C, 0x251C, 0x2500, 0x253C, 0x00E3, 0x00C3,
	0x255A, 0x2554, 0x2569, 0x2566, 0x2560, 0x2550, 0x256C, 0x00A4,
	0x00BA, 0x00AA, 0x00CA, 0x00CB, 0x00C8, 0xFFFD, 0x00CD, 0x00CE,
	0x00CF, 0x2518, 0x250C, 0x2588, 0x2584, 0x00A6, 0x00CC, 0x2580,
	0x00D3, 0x00DF, 0x00D4, 0x00D2, 0x00F5, 0x00D5, 0x00B5, 0xFFFD,
	0x00D7, 0x00DA, 0x00DB, 0x00D9, 0x00EC, 0x00FF, 0x00AF, 0x00B4,
	0x00AD, 0x00B1, 0xFFFD, 0x00BE, 0x00B6, 0x00A7, 0x00F7, 0x00B8,
	0x00B0, 0x00A8, 0x00B7, 0x00B9, 0x00B3, 0x00B2, 0x25A0, 0x00A0,
}
