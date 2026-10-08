package files

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// The text encodings Gezgin reads and writes back: UTF-8, with or without a byte order mark, and
// Windows-1254, the Turkish code page (ISO-8859-9 files read the same for their letters).
const (
	EncodingUTF8        = "utf-8"
	EncodingWindows1254 = "windows-1254"
)

// DecodeText returns raw as text and the encoding it was read in. ok is false when no supported
// encoding reads raw back unchanged; the text then has replacement characters and is only to be
// shown, never written back.
func DecodeText(raw []byte) (text, encoding string, ok bool) {
	if utf8.Valid(raw) {
		return string(raw), EncodingUTF8, true
	}

	// NUL bytes and UTF-16 byte order marks mean another kind of file, not a legacy code page.
	legacy := bytes.IndexByte(raw, 0) == -1 &&
		!bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) && !bytes.HasPrefix(raw, []byte{0xFE, 0xFF})
	if legacy {
		decoded, err := charmap.Windows1254.NewDecoder().Bytes(raw)
		if err == nil && !bytes.ContainsRune(decoded, utf8.RuneError) {
			if back, err := charmap.Windows1254.NewEncoder().Bytes(decoded); err == nil && bytes.Equal(back, raw) {
				return string(decoded), EncodingWindows1254, true
			}
		}
	}

	return strings.ToValidUTF8(string(raw), "\uFFFD"), "", false
}

// EncodeText writes text in the given encoding. A character the encoding cannot hold is refused,
// not replaced.
func EncodeText(text, encoding string) ([]byte, error) {
	switch encoding {
	case "", EncodingUTF8:
		return []byte(text), nil
	case EncodingWindows1254:
		raw, err := charmap.Windows1254.NewEncoder().Bytes([]byte(text))
		if err != nil {
			return nil, fmt.Errorf("%w: the text has characters %s cannot hold", fberrors.ErrInvalidRequestParams, encoding)
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("%w: unknown encoding %q", fberrors.ErrInvalidRequestParams, encoding)
	}
}

// Version identifies a file's content, so that a save can tell whether the file changed since it
// was opened.
func Version(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
