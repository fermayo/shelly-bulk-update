package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const scanTimeout = 60 * time.Second

type config struct {
	username string
	password string
	stage    string
	gen      int
}

// shouldUpdate reports whether a device of the given generation should be
// updated given the target generation filter (0 = all).
func shouldUpdate(isGen2Plus bool, targetGen int) bool {
	if isGen2Plus {
		return targetGen == 0 || targetGen == 2 || targetGen == 3
	}
	return targetGen == 0 || targetGen == 1
}

func updateGen1(client *shellyClient, d *display, state *deviceState, cfg config) {
	d.update(state, statusChecking, "")

	if err := client.gen1TriggerUpdateCheck(state.address); err != nil {
		d.update(state, statusFailed, err.Error())
		return
	}

	// The update check runs asynchronously on the device side.
	time.Sleep(5 * time.Second)

	status, err := client.gen1GetUpdateStatus(state.address)
	if err != nil {
		d.update(state, statusFailed, err.Error())
		return
	}

	hasUpdate := (cfg.stage == "stable" && status.HasUpdate) ||
		(cfg.stage == "beta" && status.OldVersion != status.BetaVersion)
	if !hasUpdate {
		d.update(state, statusUpToDate, status.OldVersion)
		return
	}

	fromVersion := status.OldVersion
	toVersion := status.NewVersion
	if cfg.stage == "beta" {
		toVersion = status.BetaVersion
	}
	d.update(state, statusUpdating, fmt.Sprintf("%s → %s", fromVersion, toVersion))

	status, err = client.gen1TriggerUpdate(state.address, cfg.stage == "beta")
	if err != nil {
		d.update(state, statusFailed, err.Error())
		return
	}

	for status.Status == "updating" {
		time.Sleep(5 * time.Second)
		updated, err := client.gen1GetUpdateStatus(state.address)
		if err != nil {
			continue // device may be rebooting; retry
		}
		status = updated
	}

	// After the update, OldVersion holds the newly installed firmware version.
	d.update(state, statusUpdated, fmt.Sprintf("%s → %s", fromVersion, status.OldVersion))
}

func updateGen2(client *shellyClient, d *display, state *deviceState, cfg config) {
	d.update(state, statusChecking, "")

	info, err := client.gen2CheckForUpdate(state.address)
	if err != nil {
		d.update(state, statusFailed, err.Error())
		return
	}

	targetVersion := info.Stable.Version
	if cfg.stage == "beta" {
		targetVersion = info.Beta.Version
	}
	if targetVersion == "" {
		d.update(state, statusUpToDate, "")
		return
	}

	d.update(state, statusUpdating, fmt.Sprintf("→ %s", targetVersion))

	if err := client.gen2TriggerUpdate(state.address, cfg.stage); err != nil {
		d.update(state, statusFailed, err.Error())
		return
	}

	// Poll until the available update version is empty, meaning the device has
	// rebooted with the new firmware. Allow up to ~60s for this.
	const maxPolls = 12
	for i := 0; i < maxPolls; i++ {
		time.Sleep(5 * time.Second)
		info, err := client.gen2CheckForUpdate(state.address)
		if err != nil {
			continue // device is likely rebooting
		}
		pending := info.Stable.Version
		if cfg.stage == "beta" {
			pending = info.Beta.Version
		}
		if pending == "" {
			d.update(state, statusUpdated, targetVersion)
			return
		}
	}

	d.update(state, statusFailed, "timed out waiting for update to complete")
}

func handleDevice(client *shellyClient, d *display, cfg config, name, address string, txtRecords []string) {
	isGen2Plus := slices.Contains(txtRecords, "gen=2") || slices.Contains(txtRecords, "gen=3")
	if !shouldUpdate(isGen2Plus, cfg.gen) {
		return
	}

	gen := "gen1"
	if isGen2Plus {
		gen = "gen2+"
	}

	state := d.addDevice(name, address, gen)
	if isGen2Plus {
		updateGen2(client, d, state, cfg)
	} else {
		updateGen1(client, d, state, cfg)
	}
}

func main() {
	var cfg config
	flag.StringVar(&cfg.username, "username", "admin", "username for HTTP basic auth")
	flag.StringVar(&cfg.password, "password", "", "password for HTTP basic auth")
	flag.StringVar(&cfg.stage, "stage", "stable", "firmware channel: stable or beta")
	flag.IntVar(&cfg.gen, "gen", 0, "device generation to update (0 = all, 1 = gen1, 2 or 3 = gen2+)")
	flag.Parse()

	if cfg.stage != "stable" && cfg.stage != "beta" {
		fmt.Fprintln(os.Stderr, "error: -stage must be 'stable' or 'beta'")
		flag.Usage()
		os.Exit(2)
	}

	client := newShellyClient(cfg.username, cfg.password)

	d := newDisplay(scanTimeout)
	d.start()

	// Listen only for IPv4 to avoid empty AddrIPv4 slices when IPv6 arrives first.
	// See https://github.com/grandcat/zeroconf/issues/27
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		log.Fatalln("failed to initialize mDNS resolver:", err)
	}

	var wg sync.WaitGroup
	entries := make(chan *zeroconf.ServiceEntry)

	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			if !strings.HasPrefix(strings.ToLower(entry.Instance), "shelly") {
				continue
			}
			address := entry.HostName
			if len(entry.AddrIPv4) > 0 {
				address = entry.AddrIPv4[0].String()
				// IPv6 support is limited; see https://shelly-api-docs.shelly.cloud/gen2/General/IPv6
			}
			entry := entry // capture for goroutine
			wg.Add(1)
			go func() {
				defer wg.Done()
				handleDevice(client, d, cfg, entry.Instance, address, entry.Text)
			}()
		}
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()
	if err := resolver.Browse(ctx, "_http._tcp", "local.", entries); err != nil {
		log.Fatalln("failed to browse mDNS:", err)
	}

	<-ctx.Done()
	wg.Wait()
	d.finalRender()
}
