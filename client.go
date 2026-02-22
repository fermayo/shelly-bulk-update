package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
