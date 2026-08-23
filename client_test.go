package main

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestParseDigestChallenge(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   map[string]string
	}{
		{
			name:   "typical shelly challenge",
			header: `Digest realm="shellyplus1-a1b2c3", nonce="66f0a1b2", qop="auth"`,
			want: map[string]string{
				"realm": "shellyplus1-a1b2c3",
				"nonce": "66f0a1b2",
				"qop":   "auth",
			},
		},
		{
			name:   "quoted value containing comma",
			header: `Digest realm="Shelly, Inc.", nonce="n1"`,
			want: map[string]string{
				"realm": "Shelly, Inc.",
				"nonce": "n1",
			},
		},
		{
			name:   "unquoted parameters",
			header: `Digest algorithm=SHA-256, opaque=deadbeef`,
			want: map[string]string{
				"algorithm": "SHA-256",
				"opaque":    "deadbeef",
			},
		},
		{
			name:   "missing prefix still parses",
			header: `realm="r", nonce="n"`,
			want: map[string]string{
				"realm": "r",
				"nonce": "n",
			},
		},
		{
			name:   "extra whitespace",
			header: `Digest   realm="r" ,  nonce="n"  `,
			want: map[string]string{
				"realm": "r",
				"nonce": "n",
			},
		},
		{
			name:   "empty header",
			header: "",
			want:   map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDigestChallenge(tt.header)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDigestChallenge(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestSHA256Hex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}
	for _, tt := range tests {
		if got := sha256Hex(tt.input); got != tt.want {
			t.Errorf("sha256Hex(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestBuildDigestAuthHeaderWithoutQop(t *testing.T) {
	c := newShellyClient("admin", "password")
	got := c.buildDigestAuthHeader("GET", "/rpc/Shelly.CheckForUpdate", "shelly", "abc123", "", "SHA-256")
	want := `Digest username="admin", realm="shelly", nonce="abc123", uri="/rpc/Shelly.CheckForUpdate", algorithm=SHA-256, response="07efc21111c8a8720b5f742839fb392acedc52f584d770254d4dfbc787106aab"`
	if got != want {
		t.Errorf("buildDigestAuthHeader() =\n%s\nwant:\n%s", got, want)
	}
}

func TestBuildDigestAuthHeaderWithQop(t *testing.T) {
	c := newShellyClient("admin", "password")
	got := c.buildDigestAuthHeader("GET", "/rpc/Shelly.CheckForUpdate", "shelly", "abc123", "auth", "SHA-256")

	for _, want := range []string{
		`username="admin"`,
		`realm="shelly"`,
		`nonce="abc123"`,
		`uri="/rpc/Shelly.CheckForUpdate"`,
		`algorithm=SHA-256`,
		`qop=auth`,
		`nc=00000001`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("header missing %s:\n%s", want, got)
		}
	}

	if !regexp.MustCompile(`cnonce="[0-9a-f]{16}"`).MatchString(got) {
		t.Errorf("cnonce should be 16 hex chars in quotes:\n%s", got)
	}

	if !regexp.MustCompile(`response="[0-9a-f]{64}"`).MatchString(got) {
		t.Errorf("response should be a SHA-256 hex digest:\n%s", got)
	}
}
