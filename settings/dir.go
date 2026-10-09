package settings

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/afero"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[^0-9A-Za-z@_\-.]`)

	dashes = regexp.MustCompile(`[\-]+`)
)

// HomeDir is a user's own folder, in the folder of the users' folders (Gezgin, K148).
func (s *Settings) HomeDir(username string) (string, error) {
	username = cleanUsername(username)
	if username == "" || username == "-" || username == "." {
		log.Printf("create user: invalid user for home dir creation: [%s]", username)
		return "", errors.New("invalid user for home dir creation")
	}
	return path.Join(s.UserHomeBasePath, username), nil
}

// MakeUserDir makes the user directory according to settings.
func (s *Settings) MakeUserDir(username, userScope, serverRoot string) (string, error) {
	userScope = strings.TrimSpace(userScope)
	if userScope == "" && s.CreateUserDir {
		var err error
		if userScope, err = s.HomeDir(username); err != nil {
			return "", err
		}
	}

	userScope = path.Join("/", userScope)

	// Gezgin's folders (the trash, the uploads in progress, the archive jobs) are nobody's scope:
	// no path reaches them. Letter case is ignored, as the disk may ignore it.
	if IsReserved(userScope, true) {
		return "", fmt.Errorf("%w: %s", fberrors.ErrInvalidRequestParams, reservedScope)
	}

	fs := afero.NewBasePathFs(afero.NewOsFs(), serverRoot)
	if err := fs.MkdirAll(userScope, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create user home dir: [%s]: %w", userScope, err)
	}
	return userScope, nil
}

func cleanUsername(s string) string {
	// Remove any trailing space to avoid ending on -
	s = strings.Trim(s, " ")
	s = strings.ReplaceAll(s, "..", "")

	// Replace all characters which not in the list `0-9A-Za-z@_\-.` with a dash
	s = invalidFilenameChars.ReplaceAllString(s, "-")

	// Remove any multiple dashes caused by replacements above
	s = dashes.ReplaceAllString(s, "-")
	return s
}
