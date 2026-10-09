package pack

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// output is where an archive is written, through a buffer: files on disk, where a header
// written earlier can be filled in once its data is, or a stream, where it cannot.
type output struct {
	buf   *bufio.Writer
	count int64     // the bytes written so far
	disk  *diskSink // nil for a stream
}

func newOutput(dir, name string, volume int64, limits Limits) *output {
	disk := &diskSink{dir: dir, name: name, size: volume, limits: limits}
	return &output{buf: bufio.NewWriterSize(disk, 1<<20), disk: disk}
}

func newStream(w io.Writer) *output {
	return &output{buf: bufio.NewWriterSize(w, 256<<10)}
}

func (o *output) Write(p []byte) (int, error) {
	n, err := o.buf.Write(p)
	o.count += int64(n)
	return n, err
}

func (o *output) seekable() bool { return o.disk != nil }

// writeAt writes p over what was written at off.
func (o *output) writeAt(p []byte, off int64) error {
	if err := o.buf.Flush(); err != nil {
		return err
	}
	return o.disk.writeAt(p, off)
}

func (o *output) flush() error { return o.buf.Flush() }

// finish closes the files written; when the archive is complete, it makes them durable, gives
// them their mode, and names a set that took one volume as the whole archive. It returns the
// names of the files.
func (o *output) finish(mode fs.FileMode, complete bool) ([]string, error) {
	var err error
	if complete {
		err = o.buf.Flush()
	}
	d := o.disk
	if d.cur != nil {
		if closeErr := d.cur.Close(); err == nil {
			err = closeErr
		}
		d.cur = nil
	}
	if err != nil || !complete {
		return d.files, err
	}
	for _, name := range d.files {
		if err := syncFile(fileFor(d.dir, name), mode); err != nil {
			return d.files, err
		}
	}
	if d.size > 0 && len(d.files) == 1 {
		if err := os.Rename(fileFor(d.dir, d.files[0]), fileFor(d.dir, d.name)); err != nil {
			return d.files, err
		}
		d.files[0] = d.name
	}
	return d.files, nil
}

func syncFile(p string, mode fs.FileMode) error {
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = f.Sync()
	if chmodErr := f.Chmod(mode); err == nil {
		err = chmodErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// diskSink writes an archive into dir: into the file name, or, with a size, into volumes of
// that many bytes, name.001, name.002, ... It keeps to the limits on bytes and volumes.
type diskSink struct {
	dir, name string
	size      int64
	limits    Limits
	files     []string
	cur       *os.File
	written   int64 // in all
	inCur     int64 // in the current file
}

func (d *diskSink) Write(p []byte) (int, error) {
	if d.limits.Bytes > 0 && d.written+int64(len(p)) > d.limits.Bytes {
		return 0, fail(CodeNoSpace, nil)
	}
	total := 0
	for len(p) > 0 {
		if d.cur == nil || d.size > 0 && d.inCur == d.size {
			if err := d.next(); err != nil {
				return total, err
			}
		}
		chunk := p
		if d.size > 0 && int64(len(chunk)) > d.size-d.inCur {
			chunk = chunk[:d.size-d.inCur]
		}
		n, err := d.cur.Write(chunk)
		total += n
		d.written += int64(n)
		d.inCur += int64(n)
		p = p[n:]
		if err != nil {
			return total, diskError(err)
		}
	}
	return total, nil
}

// next starts the next file.
func (d *diskSink) next() error {
	if d.cur != nil {
		if err := d.cur.Close(); err != nil {
			return diskError(err)
		}
		d.cur = nil
	}
	name := d.name
	if d.size > 0 {
		if len(d.files) >= d.limits.Volumes {
			return fail(CodeVolumes, nil)
		}
		name = fmt.Sprintf("%s.%03d", d.name, len(d.files)+1)
	}
	f, err := os.OpenFile(fileFor(d.dir, name), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return diskError(err)
	}
	d.files = append(d.files, name)
	d.cur, d.inCur = f, 0
	return nil
}

// writeAt writes p over what was written at off, in whichever files that lies.
func (d *diskSink) writeAt(p []byte, off int64) error {
	if off < 0 || off+int64(len(p)) > d.written {
		return errors.New("pack: a write past the end")
	}
	for len(p) > 0 {
		i, at, n := 0, off, int64(len(p))
		if d.size > 0 {
			i, at = int(off/d.size), off%d.size
			n = min(n, d.size-at)
		}
		if err := d.writeIn(i, p[:n], at); err != nil {
			return diskError(err)
		}
		p, off = p[n:], off+n
	}
	return nil
}

func (d *diskSink) writeIn(i int, p []byte, at int64) error {
	if i == len(d.files)-1 && d.cur != nil {
		_, err := d.cur.WriteAt(p, at)
		return err
	}
	f, err := os.OpenFile(fileFor(d.dir, d.files[i]), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteAt(p, at)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// diskError tells a full disk apart.
func diskError(err error) error {
	if errors.Is(err, syscall.ENOSPC) {
		return fail(CodeNoSpace, err)
	}
	return err
}
