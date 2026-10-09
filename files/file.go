package files

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"hash"
	"image"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/unpack"
	"github.com/spf13/afero"
)

var (
	reSubDirs = regexp.MustCompile("(?i)^sub(s|titles)$")
	reSubExts = regexp.MustCompile("(?i)(.vtt|.srt|.ass|.ssa)$")
)

// FileInfo describes a file.
type FileInfo struct {
	*Listing
	Fs         afero.Fs          `json:"-"`
	Path       string            `json:"path"`
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	Extension  string            `json:"extension"`
	ModTime    time.Time         `json:"modified"`
	Mode       os.FileMode       `json:"mode"`
	IsDir      bool              `json:"isDir"`
	IsSymlink  bool              `json:"isSymlink"`
	Type       string            `json:"type"`
	Subtitles  []string          `json:"subtitles,omitempty"`
	Content    string            `json:"content,omitempty"`
	Checksums  map[string]string `json:"checksums,omitempty"`
	Token      string            `json:"token,omitempty"`
	currentDir []os.FileInfo     `json:"-"`
	Resolution *ImageResolution  `json:"resolution,omitempty"`
	// Encoding is the encoding Content was read in, empty when no supported encoding reads the
	// file back unchanged; Version identifies the file's content (see Version).
	Encoding string `json:"encoding,omitempty"`
	Version  string `json:"version,omitempty"`
	// Count is how many items a listed folder shows, and SizeUnknown tells that Size is not its
	// total, the walk having stopped short (Gezgin, K84); both only when the listing was asked
	// for its folders' sizes.
	Count       *int `json:"count,omitempty"`
	SizeUnknown bool `json:"sizeUnknown,omitempty"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Fs         afero.Fs
	Path       string
	Modify     bool
	Expand     bool
	CalcImgRes bool
	Token      string
	Checker    rules.Checker
	Content    bool
	// DirSizes, when set, gives a listing's folders their count and size (Gezgin, K85); the size
	// includes what DirSizes allows, which the count, following Checker, may leave out.
	DirSizes rules.Checker
}

type ImageResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// NewFileInfo creates a File object from a path and a given user. This File
// object will be automatically filled depending on if it is a directory
// or a file. If it's a video file, it will also detect any subtitles.
func NewFileInfo(opts *FileOptions) (*FileInfo, error) {
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}

	file, err := stat(opts)
	if err != nil {
		return nil, err
	}

	// Do not expose the name of root directory.
	if file.Path == "/" {
		file.Name = ""
	}

	if opts.Expand {
		if file.IsDir {
			if err := file.readListing(opts.Checker, opts.DirSizes, opts.CalcImgRes); err != nil {
				return nil, err
			}
			return file, nil
		}

		err = file.detectType(opts.Modify, opts.Content, true, opts.CalcImgRes)
		if err != nil {
			return nil, err
		}
	}

	return file, err
}

func stat(opts *FileOptions) (*FileInfo, error) {
	var file *FileInfo

	if lstaterFs, ok := opts.Fs.(afero.Lstater); ok {
		info, _, err := lstaterFs.LstatIfPossible(opts.Path)
		if err != nil {
			return nil, err
		}
		file = &FileInfo{
			Fs:        opts.Fs,
			Path:      opts.Path,
			Name:      info.Name(),
			ModTime:   info.ModTime(),
			Mode:      info.Mode(),
			IsDir:     info.IsDir(),
			IsSymlink: IsSymlink(info.Mode()),
			Size:      info.Size(),
			Extension: filepath.Ext(info.Name()),
			Token:     opts.Token,
		}
	}

	// regular file
	if file != nil && !file.IsSymlink {
		return file, nil
	}

	// fs doesn't support afero.Lstater interface or the file is a symlink
	info, err := opts.Fs.Stat(opts.Path)
	if err != nil {
		// can't follow symlink
		if file != nil && file.IsSymlink {
			return file, nil
		}
		return nil, err
	}

	// set correct file size in case of symlink
	if file != nil && file.IsSymlink {
		file.Size = info.Size()
		file.IsDir = info.IsDir()
		return file, nil
	}

	file = &FileInfo{
		Fs:        opts.Fs,
		Path:      opts.Path,
		Name:      info.Name(),
		ModTime:   info.ModTime(),
		Mode:      info.Mode(),
		IsDir:     info.IsDir(),
		Size:      info.Size(),
		Extension: filepath.Ext(info.Name()),
		Token:     opts.Token,
	}

	return file, nil
}

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
func (i *FileInfo) Checksum(algo string) error {
	if i.IsDir {
		return fberrors.ErrIsDirectory
	}

	if i.Checksums == nil {
		i.Checksums = map[string]string{}
	}

	reader, err := i.Fs.Open(i.Path)
	if err != nil {
		return err
	}
	defer reader.Close()

	var h hash.Hash

	switch algo {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return fberrors.ErrInvalidOption
	}

	_, err = io.Copy(h, reader)
	if err != nil {
		return err
	}

	i.Checksums[algo] = hex.EncodeToString(h.Sum(nil))
	return nil
}

func (i *FileInfo) RealPath() string {
	if realPathFs, ok := i.Fs.(interface {
		RealPath(name string) (fPath string, err error)
	}); ok {
		realPath, err := realPathFs.RealPath(i.Path)
		if err == nil {
			return realPath
		}
	}

	return i.Path
}

func (i *FileInfo) detectType(modify, saveContent, readHeader bool, calcImgRes bool) error {
	if IsNamedPipe(i.Mode) {
		i.Type = "blob"
		return nil
	}
	// failing to detect the type should not return error.
	// imagine the situation where a file in a dir with thousands
	// of files couldn't be opened: we'd have immediately
	// a 500 even though it doesn't matter. So we just log it.

	mimetype := mime.TypeByExtension(i.Extension)

	var buffer []byte
	if readHeader {
		buffer = i.readFirstBytes()

		if mimetype == "" {
			mimetype = http.DetectContentType(buffer)
		}
	} else if unpack.IsArchive(i.Name) {
		// A listing reads no header, so an archive, or a part of a set, small enough passed for
		// text (Gezgin): it is named by the rule "Arşivi aç" follows.
		i.Type = "archive"
		return nil
	}

	switch {
	case strings.HasPrefix(mimetype, "video"):
		i.Type = "video"
		i.detectSubtitles()
		return nil
	case strings.HasPrefix(mimetype, "audio"):
		i.Type = "audio"
		return nil
	case strings.HasPrefix(mimetype, "image"):
		i.Type = "image"
		if calcImgRes {
			resolution, err := calculateImageResolution(i.Fs, i.Path)
			if err != nil {
				log.Printf("Error calculating image resolution: %v", err)
			} else {
				i.Resolution = resolution
			}
		}
		return nil
	case strings.HasSuffix(mimetype, "pdf"):
		i.Type = "pdf"
		return nil
	case !readHeader && mimetype != "" && !isTextType(mimetype):
		// Without the header, an extension that names another type is not text (Gezgin).
		i.Type = "blob"
		return nil
	case (strings.HasPrefix(mimetype, "text") || !isBinary(buffer)) && i.Size <= 10*1024*1024: // 10 MB
		i.Type = "text"

		if !modify {
			i.Type = "textImmutable"
		}

		if saveContent {
			afs := &afero.Afero{Fs: i.Fs}
			content, err := afs.ReadFile(i.Path)
			if err != nil {
				return err
			}

			var ok bool
			i.Content, i.Encoding, ok = DecodeText(content)
			i.Version = Version(content)
			// A text no supported encoding reads back unchanged would be damaged by a save.
			if !ok {
				i.Type = "textImmutable"
			}
		}
		return nil
	default:
		i.Type = "blob"
	}

	return nil
}

// textTypes are the parts of a MIME type that make it text, besides text/*.
var textTypes = []string{"json", "xml", "javascript", "ecmascript", "yaml", "toml", "x-sh",
	"shellscript", "x-csh", "sql", "subrip", "x-tex", "latex", "php", "perl", "python", "ruby"}

// isTextType reports whether a file of the MIME type mimetype is text.
func isTextType(mimetype string) bool {
	if strings.HasPrefix(mimetype, "text/") {
		return true
	}
	for _, t := range textTypes {
		if strings.Contains(mimetype, t) {
			return true
		}
	}
	return false
}

func calculateImageResolution(fSys afero.Fs, filePath string) (*ImageResolution, error) {
	file, err := fSys.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := file.Close(); cErr != nil {
			log.Printf("Failed to close file: %v", cErr)
		}
	}()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return nil, err
	}

	return &ImageResolution{
		Width:  config.Width,
		Height: config.Height,
	}, nil
}

func (i *FileInfo) readFirstBytes() []byte {
	reader, err := i.Fs.Open(i.Path)
	if err != nil {
		log.Print(err)
		i.Type = "blob"
		return nil
	}
	defer reader.Close()

	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Print(err)
		i.Type = "blob"
		return nil
	}

	return buffer[:n]
}

func (i *FileInfo) detectSubtitles() {
	if i.Type != "video" {
		return
	}

	i.Subtitles = []string{}
	ext := filepath.Ext(i.Path)

	// detect multiple languages. Base*.vtt
	parentDir := strings.TrimRight(i.Path, i.Name)
	var dir []os.FileInfo
	if len(i.currentDir) > 0 {
		dir = i.currentDir
	} else {
		var err error
		dir, err = afero.ReadDir(i.Fs, parentDir)
		if err != nil {
			return
		}
	}

	base := strings.TrimSuffix(i.Name, ext)
	for _, f := range dir {
		// load all supported subtitles from subs directories
		// should cover all instances of subtitle distributions
		// like tv-shows with multiple episodes in single dir
		if f.IsDir() && reSubDirs.MatchString(f.Name()) {
			subsDir := path.Join(parentDir, f.Name())
			i.loadSubtitles(subsDir, base, true)
		} else if isSubtitleMatch(f, base) {
			i.addSubtitle(path.Join(parentDir, f.Name()))
		}
	}
}

func (i *FileInfo) loadSubtitles(subsPath, baseName string, recursive bool) {
	dir, err := afero.ReadDir(i.Fs, subsPath)
	if err == nil {
		for _, f := range dir {
			if isSubtitleMatch(f, "") {
				i.addSubtitle(path.Join(subsPath, f.Name()))
			} else if f.IsDir() && recursive && strings.HasPrefix(f.Name(), baseName) {
				subsDir := path.Join(subsPath, f.Name())
				i.loadSubtitles(subsDir, baseName, false)
			}
		}
	}
}

func IsSupportedSubtitle(fileName string) bool {
	return reSubExts.MatchString(fileName)
}

func isSubtitleMatch(f fs.FileInfo, baseName string) bool {
	return !f.IsDir() && strings.HasPrefix(f.Name(), baseName) &&
		IsSupportedSubtitle(f.Name())
}

func (i *FileInfo) addSubtitle(fPath string) {
	i.Subtitles = append(i.Subtitles, fPath)
}

// readListing lists the folder. An entry's type comes from its extension: reading the first bytes
// of every file made a large folder slow to list (Gezgin), so the content is only looked at when
// a single file is opened.
func (i *FileInfo) readListing(checker, dirSizes rules.Checker, calcImgRes bool) error {
	dir, err := readDir(i.Fs, i.Path)
	if err != nil {
		return err
	}

	var walk *dirWalk
	if dirSizes != nil {
		walk = newDirWalk(i.Fs, dirSizes)
	}

	listing := &Listing{
		Items:    []*FileInfo{},
		NumDirs:  0,
		NumFiles: 0,
	}

	for _, f := range dir {
		name := f.Name()
		fPath := path.Join(i.Path, name)

		if !checker.Check(fPath) {
			continue
		}

		isSymlink, isInvalidLink := false, false
		if IsSymlink(f.Mode()) {
			isSymlink = true
			// It's a symbolic link. We try to follow it. The scoped filesystem
			// refuses to dereference a link whose target escapes the scope
			// (permission error); such a link is omitted from the listing
			// entirely so it cannot leak the target's metadata. Any other
			// failure means a broken link, which we surface as an invalid link
			// rather than the target's information.
			info, err := i.Fs.Stat(fPath)
			switch {
			case err == nil:
				f = info
			case errors.Is(err, os.ErrPermission):
				continue
			default:
				isInvalidLink = true
			}
		}

		file := &FileInfo{
			Fs:         i.Fs,
			Name:       name,
			Size:       f.Size(),
			ModTime:    f.ModTime(),
			Mode:       f.Mode(),
			IsDir:      f.IsDir(),
			IsSymlink:  isSymlink,
			Extension:  filepath.Ext(name),
			Path:       fPath,
			currentDir: dir,
		}

		if !file.IsDir && strings.HasPrefix(mime.TypeByExtension(file.Extension), "image/") && calcImgRes {
			resolution, err := calculateImageResolution(file.Fs, file.Path)
			if err != nil {
				log.Printf("Error calculating resolution for image %s: %v", file.Path, err)
			} else {
				file.Resolution = resolution
			}
		}

		if file.IsDir {
			listing.NumDirs++
			if walk != nil {
				file.setDirFacts(checker, walk)
			}
		} else {
			listing.NumFiles++

			if isInvalidLink {
				file.Type = "invalid_link"
			} else {
				err := file.detectType(true, false, false, calcImgRes)
				if err != nil {
					return err
				}
			}
		}

		listing.Items = append(listing.Items, file)
	}

	i.Listing = listing
	return nil
}

// ListingItems gives the items at paths as one listing would show them (Gezgin: the favourites
// page, K90): a file's type from its name, and, with dirSizes, a folder's count and size within one
// budget. An item the checker refuses, or that cannot be read, is nil.
func ListingItems(fs afero.Fs, checker, dirSizes rules.Checker, paths []string) []*FileInfo {
	var walk *dirWalk
	if dirSizes != nil {
		walk = newDirWalk(fs, dirSizes)
	}
	items := make([]*FileInfo, len(paths))
	for i, p := range paths {
		if !checker.Check(p) {
			continue
		}
		file, err := stat(&FileOptions{Fs: fs, Path: p, Checker: checker})
		if err != nil {
			continue
		}
		if file.IsDir {
			if walk != nil {
				file.setDirFacts(checker, walk)
			}
		} else if err := file.detectType(true, false, false, false); err != nil {
			continue
		}
		items[i] = file
	}
	return items
}

func readDir(afs afero.Fs, dirname string) ([]os.FileInfo, error) {
	dir, err := afero.ReadDir(afs, dirname)
	if err == nil {
		return dir, nil
	}

	dir, fallbackErr := readDirNames(afs, dirname)
	if fallbackErr != nil {
		return nil, err
	}

	return dir, nil
}

func readDirNames(afs afero.Fs, dirname string) ([]os.FileInfo, error) {
	file, err := afs.Open(dirname)
	if err != nil {
		return nil, err
	}

	names, err := file.Readdirnames(-1)
	if closeErr := file.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}

	sort.Strings(names)
	dir := make([]os.FileInfo, 0, len(names))
	for _, name := range names {
		fPath := path.Join(dirname, name)
		info, err := lstatIfPossible(afs, fPath)
		if err != nil {
			log.Printf("Skipping inaccessible file %s: %v", fPath, err)
			continue
		}

		dir = append(dir, info)
	}

	return dir, nil
}

func lstatIfPossible(afs afero.Fs, name string) (os.FileInfo, error) {
	if lstaterFs, ok := afs.(afero.Lstater); ok {
		info, _, err := lstaterFs.LstatIfPossible(name)
		return info, err
	}

	return afs.Stat(name)
}
