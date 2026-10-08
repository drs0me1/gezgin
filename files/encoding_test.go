package files

import (
	"testing"
)

// "şeker ığdır" in Windows-1254.
var turkish1254 = []byte{0xFE, 'e', 'k', 'e', 'r', ' ', 0xFD, 0xF0, 'd', 0xFD, 'r', '\r', '\n'}

func TestDecodeText(t *testing.T) {
	cases := []struct {
		name     string
		raw      []byte
		text     string
		encoding string
		ok       bool
	}{
		{"utf-8", []byte("şeker\n"), "şeker\n", EncodingUTF8, true},
		{"utf-8 with a byte order mark", []byte("\xEF\xBB\xBFa\n"), "\uFEFFa\n", EncodingUTF8, true},
		{"windows-1254", turkish1254, "şeker ığdır\r\n", EncodingWindows1254, true},
		{"a byte windows-1254 does not define", []byte{'a', 0x81, 'b'}, "a\uFFFDb", "", false},
		{"a NUL byte", []byte{'a', 0x00, 0xFE}, "a\x00\uFFFD", "", false},
		{"utf-16", []byte{0xFF, 0xFE, 'a', 0x00}, "\uFFFDa\x00", "", false},
	}
	for _, c := range cases {
		text, encoding, ok := DecodeText(c.raw)
		if text != c.text || encoding != c.encoding || ok != c.ok {
			t.Errorf("%s: DecodeText = %q, %q, %t; want %q, %q, %t", c.name, text, encoding, ok, c.text, c.encoding, c.ok)
		}
	}
}

func TestEncodeText(t *testing.T) {
	raw, err := EncodeText("şeker ığdır\r\n", EncodingWindows1254)
	if err != nil || string(raw) != string(turkish1254) {
		t.Errorf("windows-1254 = % x, %v; want % x", raw, err, turkish1254)
	}
	if raw, err := EncodeText("şeker", EncodingUTF8); err != nil || string(raw) != "şeker" {
		t.Errorf("utf-8 = %q, %v", raw, err)
	}
	if _, err := EncodeText("gülümse \U0001F600", EncodingWindows1254); err == nil {
		t.Errorf("an emoji was written in windows-1254")
	}
	if _, err := EncodeText("a", "koi8-r"); err == nil {
		t.Errorf("an unsupported encoding was accepted")
	}
}
