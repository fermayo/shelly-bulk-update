package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// ANSI color/style codes
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiCyan   = "\033[36m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
)

type deviceStatus int

const (
	statusQueued    deviceStatus = iota // discovered, not yet processed
	statusChecking                      // querying firmware info
	statusUpToDate                      // no update needed
	statusUpdating                      // update in progress
	statusUpdated                       // update applied successfully
	statusFailed                        // error occurred
)

type deviceState struct {
	name    string
	address string
	gen     string
	status  deviceStatus
	message string // version info or error detail
}

// display manages a live-updating terminal view with one row per device.
type display struct {
	mu            sync.Mutex
	devices       []*deviceState
	lastLineCount int
	startTime     time.Time
	timeout       time.Duration
	scanDone      bool
	stopCh        chan struct{}
	rendererDone  sync.WaitGroup
}

func newDisplay(timeout time.Duration) *display {
	return &display{
		startTime: time.Now(),
		timeout:   timeout,
		stopCh:    make(chan struct{}),
	}
}

// start launches the background render loop.
func (d *display) start() {
	d.rendererDone.Add(1)
	go func() {
		defer d.rendererDone.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.render()
			case <-d.stopCh:
				return
			}
		}
	}()
}

// addDevice registers a newly discovered device and returns its state handle.
// The caller uses the handle to push status updates via update().
func (d *display) addDevice(name, address, gen string) *deviceState {
	d.mu.Lock()
	defer d.mu.Unlock()
	state := &deviceState{
		name:    name,
		address: address,
		gen:     gen,
		status:  statusQueued,
	}
	d.devices = append(d.devices, state)
	return state
}

// update changes the status and message of a device. Thread-safe.
func (d *display) update(state *deviceState, status deviceStatus, message string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state.status = status
	state.message = message
}

// finalRender stops the background renderer, marks the scan complete, and
// prints the last stable frame followed by a blank line.
func (d *display) finalRender() {
	close(d.stopCh)
	d.rendererDone.Wait()
	d.mu.Lock()
	d.scanDone = true
	d.mu.Unlock()
	d.render()
	fmt.Println()
}

func (d *display) render() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Move cursor up to overwrite the previously rendered block.
	if d.lastLineCount > 0 {
		fmt.Printf("\033[%dA", d.lastLineCount)
	}

	lines := 0

	// ── Scanner header ──────────────────────────────────────────────────
	if d.scanDone {
		fmt.Printf("\r\033[K%s[Scanner]%s Done. Found %d Shelly device(s).\n",
			ansiBold, ansiReset, len(d.devices))
	} else {
		remaining := d.timeout - time.Since(d.startTime)
		if remaining < 0 {
			remaining = 0
		}
		fmt.Printf("\r\033[K%s[Scanner]%s %s Discovering Shelly devices... (%ds remaining)\n",
			ansiBold, ansiReset, spinner(time.Since(d.startTime)), int(remaining.Seconds()))
	}
	lines++

	// ── Device table ─────────────────────────────────────────────────────
	if len(d.devices) > 0 {
		fmt.Printf("\r\033[K\n")
		lines++
		fmt.Printf("\r\033[K  %s%-34s %-18s %-7s %s%s\n",
			ansiBold, "DEVICE", "ADDRESS", "GEN", "STATUS", ansiReset)
		fmt.Printf("\r\033[K  %s\n", strings.Repeat("─", 74))
		lines += 2

		// Fixed columns: 2 + 34 + 1 + 18 + 1 + 7 + 1 + 1(icon) + 1(space) = 66
		const fixedWidth = 66
		maxText := termWidth() - fixedWidth
		for _, dev := range d.devices {
			icon, color, text := formatDeviceStatus(dev)
			if maxText > 1 {
				text = truncate(text, maxText)
			}
			fmt.Printf("\r\033[K  %-34s %-18s %-7s %s%s %s%s\n",
				dev.name, dev.address, dev.gen,
				color, icon, text, ansiReset)
			lines++
		}
	}

	d.lastLineCount = lines
}

// termWidth returns the current terminal column width, defaulting to 120.
func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 120
	}
	return w
}

// truncate shortens s to at most n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// spinner returns a braille spinner frame based on elapsed time.
func spinner(elapsed time.Duration) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[int(elapsed.Milliseconds()/120)%len(frames)]
}

func formatDeviceStatus(dev *deviceState) (icon, color, text string) {
	switch dev.status {
	case statusQueued:
		return "●", ansiDim, "queued"
	case statusChecking:
		return "◌", ansiCyan, "checking for updates..."
	case statusUpToDate:
		if dev.message != "" {
			return "✓", ansiGreen, fmt.Sprintf("up to date (%s)", dev.message)
		}
		return "✓", ansiGreen, "up to date"
	case statusUpdating:
		return "↻", ansiYellow, fmt.Sprintf("updating %s...", dev.message)
	case statusUpdated:
		return "✓", ansiGreen, fmt.Sprintf("updated! (%s)", dev.message)
	case statusFailed:
		return "✗", ansiRed, fmt.Sprintf("failed: %s", dev.message)
	default:
		return "?", ansiDim, dev.message
	}
}
