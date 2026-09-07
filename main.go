// Command wfdrive is a headless inspector/driver for Waterfox and Firefox,
// speaking WebDriver BiDi. Waterfox 6.6 (Firefox ESR 128+) dropped the CDP
// Remote Agent but still speaks BiDi, so this drives the browser over the BiDi
// websocket directly — no geckodriver, no Selenium.
//
// The bidi package, which is what other programs import, still needs only
// github.com/coder/websocket. cobra and calvin/clihelp are this command's, for
// the help menu, and nothing importing bidi compiles them.
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

	"github.com/spf13/cobra"

	"github.com/0magnet/calvin/clihelp"
	"github.com/0magnet/wfdrive/bidi"
)

var rootCmd = &cobra.Command{
	Use:                   "wfdrive",
	Short:                 "headless inspector and driver for Waterfox / Firefox, over WebDriver BiDi",
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
}

// serveCmd keeps the positional shape the tool always had. The arguments are
// positional rather than flags because that is how it is already invoked, from
// scripts and from skywire, and a driver that changed its own call signature to
// gain a help menu would be a poor trade.
var serveCmd = &cobra.Command{
	Use:   "serve <bidiPort> <ctrlAddr> [tabURLSubstring]",
	Short: "run the persistent driver",
	Long: "Run the persistent driver: one long-lived BiDi session and tab, driven\n" +
		"over a small local HTTP control port.\n\n" +
		"Firefox permits one active BiDi session and does not release it when the\n" +
		"owning socket drops, so this stays up rather than reconnecting per call.",
	Args: cobra.RangeArgs(2, 3),
	RunE: func(_ *cobra.Command, args []string) error {
		port, ctrlAddr := args[0], args[1]
		// The optional third argument re-attaches to a tab that is already
		// open, matched on a substring of its URL, instead of opening a blank
		// one — how a driver that died gets its tab back rather than
		// abandoning it.
		tab := ""
		if len(args) > 2 {
			tab = args[2]
		}
		return bidi.Serve(context.Background(), port, ctrlAddr, tab, func(t string) {
			fmt.Printf("wfdrive serving control on http://%s (BiDi :%s, tab %s)\n", ctrlAddr, port, t)
		})
	},
}

func main() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	clihelp.Init(rootCmd, "wfdrive", true)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
