package share

type CreateBody struct {
	Password string `json:"password"`
	Expires  string `json:"expires"`
	Unit     string `json:"unit"`
	// Gezgin: a WebDAV share names its kind, the username its password goes with and whether it
	// may be written to.
	Kind     string `json:"kind"`
	Username string `json:"webdavUser"`
	Writable bool   `json:"writable"`
}

// KindWebDAV is a share served on the WebDAV port (Gezgin) rather than as a link to a page.
const KindWebDAV = "webdav"

// Link is the information needed to build a shareable link.
type Link struct {
	Hash         string `json:"hash" storm:"id,index"`
	Path         string `json:"path" storm:"index"`
	UserID       uint   `json:"userID"`
	Expire       int64  `json:"expire"`
	PasswordHash string `json:"password_hash,omitempty"`
	// Token is a random value that will only be set when PasswordHash is set. It is
	// URL-Safe and is used to download links in password-protected shares via a
	// query arg.
	Token string `json:"token,omitempty"`
	// Kind is empty for a link to a page and KindWebDAV for a folder served on the
	// WebDAV port to Username with the password, which Writable lets change it
	// (Gezgin).
	Kind     string `json:"kind,omitempty"`
	Username string `json:"username,omitempty"`
	Writable bool   `json:"writable,omitempty"`
}
