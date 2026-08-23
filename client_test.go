package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestBuildDigestAuthHeaderMD5(t *testing.T) {
	c := newShellyClient("admin", "password")

	t.Run("md5 challenge uses md5 hashes", func(t *testing.T) {
		got := c.buildDigestAuthHeader("GET", "/rpc/Shelly.CheckForUpdate", "shelly", "abc123", "", "MD5")
		want := `Digest username="admin", realm="shelly", nonce="abc123", uri="/rpc/Shelly.CheckForUpdate", algorithm=MD5, response="e1b1a5fcd4e84659fe8f812601fe4a63"`
		if got != want {
			t.Errorf("buildDigestAuthHeader() =\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("algorithm token is case-insensitive", func(t *testing.T) {
		got := c.buildDigestAuthHeader("GET", "/rpc/Shelly.CheckForUpdate", "shelly", "abc123", "", "md5")
		want := `Digest username="admin", realm="shelly", nonce="abc123", uri="/rpc/Shelly.CheckForUpdate", algorithm=md5, response="e1b1a5fcd4e84659fe8f812601fe4a63"`
		if got != want {
			t.Errorf("buildDigestAuthHeader() =\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("unknown algorithm falls back to sha256", func(t *testing.T) {
		got := c.buildDigestAuthHeader("GET", "/rpc/Shelly.CheckForUpdate", "shelly", "abc123", "", "SHA-512-256")
		want := `Digest username="admin", realm="shelly", nonce="abc123", uri="/rpc/Shelly.CheckForUpdate", algorithm=SHA-512-256, response="07efc21111c8a8720b5f742839fb392acedc52f584d770254d4dfbc787106aab"`
		if got != want {
			t.Errorf("buildDigestAuthHeader() =\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("qop auth path also uses md5", func(t *testing.T) {
		got := c.buildDigestAuthHeader("GET", "/rpc/Shelly.CheckForUpdate", "shelly", "abc123", "auth", "MD5")
		if !strings.Contains(got, `algorithm=MD5`) {
			t.Errorf("header missing algorithm=MD5:\n%s", got)
		}
		responseRe := regexp.MustCompile(`response="[0-9a-f]{32}"`)
		if !responseRe.MatchString(got) {
			t.Errorf("response should be an MD5 hex digest:\n%s", got)
		}
	})
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

// TestGetDigestAuthMD5 exercises the full 401-challenge-retry flow in get()
// against a fake device that challenges with algorithm=MD5, as older Gen2
// firmware does.
func TestGetDigestAuthMD5(t *testing.T) {
	var challenges, authorized int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Digest ") {
			challenges++
			w.Header().Set("WWW-Authenticate",
				`Digest realm="shelly", nonce="abc123", qop="auth", algorithm=MD5`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		authorized++
		if !strings.Contains(auth, `algorithm=MD5`) {
			t.Errorf("client did not honor the MD5 challenge:\n%s", auth)
		}
		if !strings.Contains(auth, `uri="/ota"`) {
			t.Errorf("digest uri should match the request path:\n%s", auth)
		}
		fmt.Fprint(w, `{"status":"idle"}`)
	}))
	defer srv.Close()

	c := newShellyClient("admin", "password")
	body, err := c.get(srv.URL + "/ota")
	if err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if challenges != 1 || authorized != 1 {
		t.Errorf("expected one 401 challenge and one authorized retry, got %d and %d", challenges, authorized)
	}
	if !strings.Contains(string(body), "idle") {
		t.Errorf("unexpected body: %s", body)
	}
}
