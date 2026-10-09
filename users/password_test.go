package users

import (
	"errors"
	"strings"
	"testing"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// K66: the minimum counts letters, and a password bcrypt cannot take is refused with its own
// error instead of bcrypt's.
func TestValidateAndHashPwdLength(t *testing.T) {
	var short fberrors.ErrShortPassword
	if _, err := ValidateAndHashPwd("ğüşıöçĞÜ", 8); err != nil {
		t.Errorf("8 Turkish letters (16 bytes) with a minimum of 8: %v", err)
	}
	if _, err := ValidateAndHashPwd("ğüşıöçĞ", 8); !errors.As(err, &short) {
		t.Errorf("7 Turkish letters (14 bytes) with a minimum of 8: %v; want ErrShortPassword", err)
	}
	if _, err := ValidateAndHashPwd(strings.Repeat("a", MaxPasswordBytes), 8); err != nil {
		t.Errorf("72 bytes: %v", err)
	}
	if _, err := ValidateAndHashPwd(strings.Repeat("ğ", 37), 32); !errors.Is(err, fberrors.ErrPasswordTooLong) {
		t.Errorf("37 letters in 74 bytes: %v; want ErrPasswordTooLong", err)
	}
}
