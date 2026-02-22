package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

const (
	// https://shelly-api-docs.shelly.cloud/gen1/#ota
	gen1OTAURLFormat = "http://%s/ota"

	// https://shelly-api-docs.shelly.cloud/gen1/#ota-check
	gen1OTACheckURLFormat = "http://%s/ota/check"

	// https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellycheckforupdate
	gen2CheckURLFormat = "http://%s/rpc/Shelly.CheckForUpdate"

	// https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyupdate
	gen2UpdateURLFormat = "http://%s/rpc/Shelly.Update?stage=%s"
)

type versionInfo struct {
	Version string `json:"version"`
	BuildID string `json:"build_id"`
}

// gen2UpdateInfo is the response from Shelly.CheckForUpdate on gen2+ devices.
// Non-empty Version means an update is available for that channel.
type gen2UpdateInfo struct {
	Stable versionInfo `json:"stable"`
	Beta   versionInfo `json:"beta"`
}

// gen1UpdateStatus is the response from the /ota endpoint on gen1 devices.
type gen1UpdateStatus struct {
	Status      string `json:"status"`
	HasUpdate   bool   `json:"has_update"`
	NewVersion  string `json:"new_version"`
	OldVersion  string `json:"old_version"`
	BetaVersion string `json:"beta_version"`
}

type shellyClient struct {
	http     *http.Client
	username string
	password string
}

func newShellyClient(username, password string) *shellyClient {
	return &shellyClient{
		http:     &http.Client{Timeout: 30 * time.Second},
		username: username,
		password: password,
	}
}

func parseDigestChallenge(header string) map[string]string {
	params := make(map[string]string)
	header = strings.TrimPrefix(header, "Digest ")
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		eqIdx := strings.Index(part, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(part[:eqIdx])
		val := strings.Trim(strings.TrimSpace(part[eqIdx+1:]), `"`)
		params[key] = val
	}
	return params
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (c *shellyClient) buildDigestAuthHeader(method, uri, realm, nonce, qop, algorithm string) string {
	ha1 := sha256Hex(c.username + ":" + realm + ":" + c.password)
	ha2 := sha256Hex(method + ":" + uri)
	if algorithm == "" {
		algorithm = "SHA-256"
	}
	if qop == "auth" {
		nc := "00000001"
		cnonceBytes := make([]byte, 8)
		rand.Read(cnonceBytes)
		cnonce := hex.EncodeToString(cnonceBytes)
		response := sha256Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
		return fmt.Sprintf(
			`Digest username="%s", realm="%s", nonce="%s", uri="%s", algorithm=%s, qop=%s, nc=%s, cnonce="%s", response="%s"`,
			c.username, realm, nonce, uri, algorithm, qop, nc, cnonce, response,
		)
	}
	response := sha256Hex(ha1 + ":" + nonce + ":" + ha2)
	return fmt.Sprintf(
		`Digest username="%s", realm="%s", nonce="%s", uri="%s", algorithm=%s, response="%s"`,
		c.username, realm, nonce, uri, algorithm, response,
	)
}

func (c *shellyClient) get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	// Gen2+ devices use Digest authentication (SHA-256) rather than Basic auth.
	// If we get a 401 with a Digest challenge, retry with proper digest credentials.
	if resp.StatusCode == http.StatusUnauthorized && c.password != "" {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()
		if strings.HasPrefix(wwwAuth, "Digest ") {
			params := parseDigestChallenge(wwwAuth)
			parsedURL, err := neturl.Parse(url)
			if err != nil {
				return nil, err
			}
			req2, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			req2.Header.Set("Authorization", c.buildDigestAuthHeader(
				http.MethodGet, parsedURL.RequestURI(),
				params["realm"], params["nonce"], params["qop"], params["algorithm"],
			))
			resp, err = c.http.Do(req2)
			if err != nil {
				return nil, err
			}
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *shellyClient) gen1TriggerUpdateCheck(address string) error {
	_, err := c.get(fmt.Sprintf(gen1OTACheckURLFormat, address))
	return err
}

func (c *shellyClient) gen1GetUpdateStatus(address string) (*gen1UpdateStatus, error) {
	body, err := c.get(fmt.Sprintf(gen1OTAURLFormat, address))
	if err != nil {
		return nil, err
	}
	var s gen1UpdateStatus
	return &s, json.Unmarshal(body, &s)
}

// gen1TriggerUpdate starts a firmware update. Pass beta=true to install the beta channel.
func (c *shellyClient) gen1TriggerUpdate(address string, beta bool) (*gen1UpdateStatus, error) {
	url := gen1OTAURLFormat
	if beta {
		url += "?beta=1"
	} else {
		url += "?update=1"
	}
	body, err := c.get(fmt.Sprintf(url, address))
	if err != nil {
		return nil, err
	}
	var s gen1UpdateStatus
	return &s, json.Unmarshal(body, &s)
}

func (c *shellyClient) gen2CheckForUpdate(address string) (*gen2UpdateInfo, error) {
	body, err := c.get(fmt.Sprintf(gen2CheckURLFormat, address))
	if err != nil {
		return nil, err
	}
	var info gen2UpdateInfo
	return &info, json.Unmarshal(body, &info)
}

func (c *shellyClient) gen2TriggerUpdate(address, stage string) error {
	_, err := c.get(fmt.Sprintf(gen2UpdateURLFormat, address, stage))
	return err
}
