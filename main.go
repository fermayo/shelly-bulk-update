package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

var scanTimeout = time.Second * 60

type shellyUpdateStatusResponse struct {
	Status     string `json:"status"`
	HasUpdate  bool   `json:"has_update"`
	NewVersion string `json:"new_version"`
	OldVersion string `json:"old_version"`
}

func makeShellyUpdateRequest(hostname string, update bool) (*shellyUpdateStatusResponse, error) {
	url := "http://%s/ota"

	if update {
		url += "?update=1"
	}

	resp, err := http.Get(fmt.Sprintf(url, hostname))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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

func triggerShellyUpdate(hostname string) (*shellyUpdateStatusResponse, error) {
	return makeShellyUpdateRequest(hostname, true)
}

func checkShellyUpdateStatus(hostname string) (*shellyUpdateStatusResponse, error) {
	return makeShellyUpdateRequest(hostname, false)
}

func updateShelly(instance *zeroconf.ServiceEntry, wg *sync.WaitGroup) {
	defer wg.Done()

	shellyAddress := instance.AddrIPv4[0].String()

	updateStatus, err := checkShellyUpdateStatus(shellyAddress)
	if err != nil {
		fmt.Printf("[%s] failed to query update status: %s, aborting...\n", instance.Instance, err)
		return
	}

	if updateStatus.HasUpdate {
		fmt.Printf(
			"[%s] update available! (%s -> %s), updating...\n",
			instance.HostName, updateStatus.OldVersion,
			updateStatus.NewVersion,
		)

		updateStatus, err := triggerShellyUpdate(shellyAddress)
		if err != nil {
			fmt.Printf("[%s] failed to start update: %s, aborting...\n", instance.Instance, err)
			return
		}

		for updateStatus.Status == "updating" {
			time.Sleep(time.Second * 5)
			updateStatusCheck, err := checkShellyUpdateStatus(shellyAddress)
			if err != nil {
				fmt.Printf("[%s] failed to query update status: %s, retrying...\n", instance.Instance, err)
			} else {
				updateStatus = updateStatusCheck
			}
		}

		fmt.Printf("[%s] device updated to %s!\n", instance.Instance, updateStatus.OldVersion)
	} else {
		fmt.Printf("[%s] already up to date (%s)\n", instance.Instance, updateStatus.OldVersion)
	}
}

func main() {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	var wg sync.WaitGroup
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		fmt.Printf("[scanner] looking for Shelly devices using mDNS (%ds timeout)...\n", int(scanTimeout.Seconds()))
		for entry := range results {
			if strings.HasPrefix(entry.Instance, "shelly") {
				wg.Add(1)
				go updateShelly(entry, &wg)
			}
		}
		fmt.Println("[scanner] scanning process finished")
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()
	err = resolver.Browse(ctx, "_http._tcp", "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()
	wg.Wait()
}
