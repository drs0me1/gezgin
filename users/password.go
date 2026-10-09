package users

import (
	"crypto/rand"
	"encoding/base64"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// MaxPasswordBytes is the longest password bcrypt takes.
const MaxPasswordBytes = 72

// ValidateAndHashPwd validates and hashes a password.
func ValidateAndHashPwd(password string, minimumLength uint) (string, error) {
	// The minimum counts letters (Gezgin): "ğ" is one, though it takes two bytes.
	if uint(utf8.RuneCountInString(password)) < minimumLength {
		return "", fberrors.ErrShortPassword{MinimumLength: minimumLength}
	}
	if len(password) > MaxPasswordBytes {
		return "", fberrors.ErrPasswordTooLong
	}

	if _, ok := commonPasswords[password]; ok {
		return "", fberrors.ErrEasyPassword
	}

	return HashPwd(password)
}

// HashPwd hashes a password.
func HashPwd(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPwd checks if a password is correct.
func CheckPwd(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func RandomPwd(passwordLength uint) (string, error) {
	randomPasswordBytes := make([]byte, passwordLength)
	var _, err = rand.Read(randomPasswordBytes)
	if err != nil {
		return "", err
	}

	// This is done purely to make the password human-readable
	var randomPasswordString = base64.URLEncoding.EncodeToString(randomPasswordBytes)
	return randomPasswordString, nil
}

// NewSecurityStamp returns a random stamp; see User.SecurityStamp.
func NewSecurityStamp() (string, error) {
	stamp := make([]byte, 16)
	if _, err := rand.Read(stamp); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(stamp), nil
}
