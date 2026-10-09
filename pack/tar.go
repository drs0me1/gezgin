package pack

import (
	"archive/tar"
	"io"

	"github.com/klauspost/compress/gzip"
)

// tarWriter writes a tar archive, through a compressor when it has one.
type tarWriter struct {
	tw   *tar.Writer
	comp io.WriteCloser
}

func newTarWriter(out *output, compress func(io.Writer) (io.WriteCloser, error)) (*tarWriter, error) {
	t := &tarWriter{}
	var w io.Writer = out
	if compress != nil {
		c, err := compress(out)
		if err != nil {
			return nil, err
		}
		t.comp, w = c, c
	}
	t.tw = tar.NewWriter(w)
	return t, nil
}

// header describes an entry as a tar does, with no owner: a file is 0644, or 0755 when it
// could be run, and a folder 0755.
func (t *tarWriter) header(e entry) *tar.Header {
	h := &tar.Header{Name: e.name, Mode: 0o644, ModTime: e.info.ModTime(), Typeflag: tar.TypeReg, Size: e.info.Size()}
	switch {
	case e.info.IsDir():
		h.Mode, h.Typeflag, h.Size = 0o755, tar.TypeDir, 0
	case e.info.Mode()&0o111 != 0:
		h.Mode = 0o755
	}
	return h
}

func (t *tarWriter) dir(e entry) error {
	return t.tw.WriteHeader(t.header(e))
}

func (t *tarWriter) file(e entry, data func(io.Writer) error) error {
	if err := t.tw.WriteHeader(t.header(e)); err != nil {
		return err
	}
	return data(t.tw)
}

func (t *tarWriter) close() error {
	if err := t.tw.Close(); err != nil {
		return err
	}
	if t.comp != nil {
		return t.comp.Close()
	}
	return nil
}

// Gzip compresses a tar's stream as tar.gz, on one core.
func Gzip(w io.Writer) (io.WriteCloser, error) {
	return gzip.NewWriterLevel(w, 5)
}
