// Command wfdrive is a headless inspector/driver for Waterfox and Firefox,
// speaking WebDriver BiDi. Waterfox 6.6 (Firefox ESR 128+) dropped the CDP
// Remote Agent but still speaks BiDi, so this drives the browser over the BiDi
// websocket directly — no geckodriver, no Selenium, one dependency
// (github.com/coder/websocket).
//
// Firefox BiDi permits ONE active session and does NOT release it when the owning
// socket drops, so one-shot invocations churn into "Maximum number of active
// sessions". wfdrive therefore runs as a PERSISTENT driver (like geckodriver):
// one long-lived session + tab, driven over a tiny local HTTP control port.
//
// Attach to a Waterfox started with:
//
//	waterfox --remote-debugging-port=9223 --remote-allow-origins=*
//
// Serve mode (persistent — run in the background, then curl it):
//
//	wfdrive serve 9223 127.0.0.1:9224
//	curl 'http://127.0.0.1:9224/nav?url=https://magnetosphere.net/&wait=10'
//	curl 'http://127.0.0.1:9224/nav?url=...&eval=<js>'
//	curl 'http://127.0.0.1:9224/eval?expr=<js>'   # evaluate, no navigation
//	curl 'http://127.0.0.1:9224/shoot'            # screenshot as it stands
//	curl  http://127.0.0.1:9224/health            # session + tab usable?
//	curl  http://127.0.0.1:9224/quit              # session.end + exit
//
// Captures land in the temp directory under names wfdrive picks itself
// (wfdrive-<n>.png / .console.txt); the response says where each one went. The
// "out" parameter is accepted and ignored — letting a request name a file it
// writes is a path-injection question nobody needs to have, on a tool whose
// callers only ever needed to know where the file is.
//
// /nav, /shoot and /eval capture a screenshot plus the page console and return
// JSON. Splitting navigation, evaluation and capture apart matters for anything
// that has to settle between steps: /nav evaluates immediately before it shoots,
// so a test that needs to toggle something and wait would otherwise have to cram
// the whole sequence into one expression.
//
// The driver heals itself. Closing its tab in the browser, or losing the
// socket, used to brick it until a restart — and restarting is precisely what
// strands the single session, so a closed tab could cost a browser restart.
// Every operation now runs through the recovery layer in package bidi, which
// re-opens the tab (and re-establishes the session if the connection went too)
// and retries once.
//
// The driver itself lives in github.com/0magnet/wfdrive/bidi so other programs
// can embed it rather than shelling out; this command is a thin wrapper.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/0magnet/wfdrive/bidi"
)

func serveMode() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: wfdrive serve <bidiPort> <ctrlAddr> [tabURLSubstring]")
	}
	port, ctrlAddr := os.Args[2], os.Args[3]
	// An optional third argument re-attaches to a tab that is already open,
	// matched on a substring of its URL, instead of opening a blank one — how a
	// driver that died gets its tab back rather than abandoning it.
	tab := ""
	if len(os.Args) > 4 {
		tab = os.Args[4]
	}
	return bidi.Serve(context.Background(), port, ctrlAddr, tab, func(t string) {
		fmt.Printf("wfdrive serving control on http://%s (BiDi :%s, tab %s)\n", ctrlAddr, port, t)
	})
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "serve" {
		if err := serveMode(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "usage: wfdrive serve <bidiPort> <ctrlAddr> [tabURLSubstring]   (persistent driver)")
	os.Exit(2)
}
