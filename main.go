package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	// https://shelly-api-docs.shelly.cloud/gen1/#ota
	otaUrl = "http://%s/ota"

	// https://shelly-api-docs.shelly.cloud/gen1/#ota-check
	otaCheckUrl = "http://%s/ota/check"

	// https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellycheckforupdate
	checkForUpdateUrl = "http://%s/rpc/Shelly.CheckForUpdate"

	// https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly#shellyupdate
	updateUrl = "http://%s/rpc/Shelly.Update?stage=%s"
)

var (
	scanTimeout = time.Second * 60
	username    string
	password    string
	updateStage string
	genToUpdate int
)

type (
	versionInfo struct {
		Version string `json:"version"`
		BuildId string `json:"build_id"`
	}

	checkForUpdateResponse struct {
		Stable versionInfo `json:"stable"`
		Beta   versionInfo `json:"beta"`
	}

	shellyUpdateStatusResponse struct {
		Status      string `json:"status"`
		HasUpdate   bool   `json:"has_update"`
		NewVersion  string `json:"new_version"`
		OldVersion  string `json:"old_version"`
		BetaVersion string `json:"beta_version"`
	}

	shellyUpdateCheckResponse struct {
		Status string `json:"status"`
	}
)

func makeGetRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func makeShellyUpdateRequest(hostname string, update bool) (*shellyUpdateStatusResponse, error) {
	url := otaUrl
	if update {
		if updateStage == "beta" {
			url += "?beta=1"
		} else {
			url += "?update=1"
		}
	}

	body, err := makeGetRequest(fmt.Sprintf(url, hostname))
	if err != nil {
		return nil, err
	}

	var updateStatus *shellyUpdateStatusResponse
	err = json.Unmarshal(body, &updateStatus)
	if err != nil {
		return nil, err
	}

	return updateStatus, nil
}

func triggerShellyUpdateCheck(hostname string) (*shellyUpdateCheckResponse, error) {
	body, err := makeGetRequest(fmt.Sprintf(otaCheckUrl, hostname))
	if err != nil {
		return nil, err
	}

	var checkStatus *shellyUpdateCheckResponse
	err = json.Unmarshal(body, &checkStatus)
	if err != nil {
		return nil, err
	}

	return checkStatus, nil
}

func makeGen2CheckForUpdateRequest(hostname string) (*checkForUpdateResponse, error) {
	body, err := makeGetRequest(fmt.Sprintf(checkForUpdateUrl, hostname))
	if err != nil {
		return nil, err
	}

	var checkForUpdate *checkForUpdateResponse
	err = json.Unmarshal(body, &checkForUpdate)
	if err != nil {
		return nil, err
	}

	return checkForUpdate, nil
}

func makeGen2UpdateRequest(hostname string, stage string) error {
	_, err := makeGetRequest(fmt.Sprintf(updateUrl, hostname, stage))
	if err != nil {
		return err
	}

	return nil
}

func triggerShellyUpdate(hostname string) (*shellyUpdateStatusResponse, error) {
	return makeShellyUpdateRequest(hostname, true)
}

func checkShellyUpdateStatus(hostname string) (*shellyUpdateStatusResponse, error) {
	return makeShellyUpdateRequest(hostname, false)
}

func updateShellyGen1(name, address string) {
	prefix := fmt.Sprintf("[%s/%s/gen1]", name, address)
	// First, we trigger a check for updates
	fmt.Printf("%s checking for updates...\n", prefix)
	_, err := triggerShellyUpdateCheck(address)
	if err != nil {
		fmt.Printf("%s failed to check for updates: %s, aborting...\n", prefix, err)
		return
	}

	// Check for updates is asynchronous, so we need to wait a bit
	time.Sleep(time.Second * 5)

	// Then, we check if there are any updates available
	updateStatus, err := checkShellyUpdateStatus(address)
	if err != nil {
		fmt.Printf("%s failed to query update status: %s, aborting...\n", prefix, err)
		return
	}

	// If there's an update available, trigger the update
	if (updateStage == "stable" && updateStatus.HasUpdate) ||
		(updateStage == "beta" && updateStatus.OldVersion != updateStatus.BetaVersion) {
		newVersion := updateStatus.NewVersion
		if updateStage == "beta" {
			newVersion = updateStatus.BetaVersion
		}
		fmt.Printf(
			"%s update available! (%s -> %s), updating...\n",
			prefix, updateStatus.OldVersion, newVersion,
		)

		updateStatus, err := triggerShellyUpdate(address)
		if err != nil {
			fmt.Printf("%s failed to start update: %s, aborting...\n", prefix, err)
			return
		}

		for updateStatus.Status == "updating" {
			time.Sleep(time.Second * 5)
			updateStatusCheck, err := checkShellyUpdateStatus(address)
			if err != nil {
				fmt.Printf("%s failed to query update status: %s, retrying...\n", prefix, err)
				continue
			}
			updateStatus = updateStatusCheck
		}

		fmt.Printf("%s device updated to %s!\n", prefix, updateStatus.OldVersion)
	} else {
		fmt.Printf("%s already up to date (%s)\n", prefix, updateStatus.OldVersion)
	}
}

func updateShellyGen2(name, address string) {
	prefix := fmt.Sprintf("[%s/%s/gen2]", name, address)
	// First, we trigger a check for updates
	fmt.Printf("%s checking for updates...\n", prefix)
	updates, err := makeGen2CheckForUpdateRequest(address)
	if err != nil {
		fmt.Printf("%s failed to check for updates: %s, aborting...\n", prefix, err)
		return
	}

	updateVersion := updates.Stable.Version
	if updateStage == "beta" {
		updateVersion = updates.Beta.Version
	}
	if updateVersion == "" {
		fmt.Printf("%s already up to date\n", prefix)
		return
	}
	newVersion := updateVersion

	fmt.Printf("%s updating to version %s...\n", prefix, updateVersion)
	err = makeGen2UpdateRequest(address, updateStage)
	if err != nil {
		fmt.Printf("%s failed to update: %s, aborting...\n", prefix, err)
		return
	}

	// wait for update to complete
	tries := 0
	for updateVersion != "" {
		tries++
		if tries > 12 {
			fmt.Printf("%s failed to check if update completed successfully", prefix)
			return
		}
		time.Sleep(time.Second * 5)
		updates, err := makeGen2CheckForUpdateRequest(address)
		if err != nil {
			fmt.Printf("%s failed to query update status: %s, retrying...\n", prefix, err)
			continue
		}

		if updateStage == "beta" {
			updateVersion = updates.Beta.Version
		} else {
			updateVersion = updates.Stable.Version
		}
	}
	fmt.Printf("%s device updated to %s!\n", prefix, newVersion)
}

func updateShelly(name, address string, txtRecords []string, genToUpdate int) {
	if slices.Contains(txtRecords, "gen=2") {
		if genToUpdate == 2 || genToUpdate == 0 {
			updateShellyGen2(name, address)
		}
		return
	}

	if genToUpdate == 1 || genToUpdate == 0 {
		updateShellyGen1(name, address)
	}
}

func main() {
	flag.StringVar(&username, "username", "admin", "username to use for authentication")
	flag.StringVar(&password, "password", "", "password to use for authentication")
	flag.StringVar(&updateStage, "stage", "stable", "stable or beta")
	flag.IntVar(&genToUpdate, "gen", 0, "device generation to update (default: all)")

	flag.Parse()

	if password != "" {
		fmt.Printf("Using basic authentication: %s:*******\n", username)
	}

	if updateStage != "stable" && updateStage != "beta" {
		flag.Usage()
		os.Exit(2)
	}

	// Listen only for IPv4 addresses. Otherwise, it may happen that ServiceEntry has an empty
	// AddrIPv4 slice. It happens when the IPv6 arrives first and ServiceEntries are not updated
	// when more data arrives.
	// See https://github.com/grandcat/zeroconf/issues/27
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	var wg sync.WaitGroup
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		fmt.Printf("[scanner] looking for Shelly devices using mDNS (%ds timeout)...\n", int(scanTimeout.Seconds()))
		for entry := range results {
			entry := entry
			if strings.HasPrefix(strings.ToLower(entry.Instance), "shelly") {
				wg.Add(1)
				go func() {
					address := entry.HostName
					if len(entry.AddrIPv4) > 0 {
						address = entry.AddrIPv4[0].String()
						// IPv6 support is still very limited
						// See https://shelly-api-docs.shelly.cloud/gen2/General/IPv6
						//} else if len(entry.AddrIPv6) > 0 {
						//	address = fmt.Sprintf("[%s]", entry.AddrIPv6[0].String())
					}
					updateShelly(entry.Instance, address, entry.Text, genToUpdate)
					wg.Done()
				}()
			}
		}
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()
	err = resolver.Browse(ctx, "_http._tcp", "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()
	fmt.Println("[scanner] scanning process finished")
	wg.Wait()
}
