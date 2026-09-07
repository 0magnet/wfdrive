# wfdrive

A headless inspector and driver for **Waterfox / Firefox**, speaking WebDriver
BiDi directly. No geckodriver, no Selenium, one dependency.

Waterfox 6.6 (Firefox ESR 128+) dropped the CDP Remote Agent, so the usual
chromedp-shaped tooling no longer works against it. Firefox does still speak
[WebDriver BiDi](https://w3c.github.io/webdriver-bidi/), and that endpoint is
built into the browser — you do not need a driver binary in the loop.

## The problem it actually solves

Firefox BiDi permits **one** active session and does **not** release it when the
owning socket drops. One-shot invocations therefore churn into `Maximum number
of active sessions` and you end up restarting the browser between commands.

So wfdrive runs as a *persistent* driver, the way geckodriver does: one
long-lived session and tab, driven over a tiny local HTTP control port. Every
operation is wrapped in a recovery layer that classifies dropped-session errors
(`no such frame`, `invalid session id`, broken pipe, EOF, …) and repairs in
place — reopen the tab, and if that fails, redial, re-create the session,
reopen the tab, retry once. Closing a tab by hand does not cost you a browser
restart.

## Install

```
go get github.com/0magnet/wfdrive
go install github.com/0magnet/wfdrive@latest
```

## Use

Start the browser with BiDi enabled:

```
waterfox --remote-debugging-port=9223 --remote-allow-origins=*
```

Run the driver in the background, then curl it:

```
wfdrive serve 9223 127.0.0.1:9224

curl 'http://127.0.0.1:9224/nav?url=https://example.com/&wait=10'
curl 'http://127.0.0.1:9224/nav?url=https://example.com/&eval=<js>'
curl 'http://127.0.0.1:9224/eval?expr=document.title'
curl 'http://127.0.0.1:9224/shoot'
curl  http://127.0.0.1:9224/health
curl  http://127.0.0.1:9224/quit
```

| Endpoint | Does |
|---|---|
| `/nav` | `browsingContext.navigate` with `wait: complete`, optional JS after load |
| `/eval` | `script.evaluate` with `awaitPromise`, no navigation |
| `/shoot` | `browsingContext.captureScreenshot`, decoded to PNG |
| `/health` | `browsingContext.getTree` — is the session and tab still usable |
| `/quit` | `session.end`, then exit |

Console output is captured continuously from `log.entryAdded` and written
alongside each capture.

Captures land in the temp directory under names wfdrive picks itself
(`wfdrive-<n>.png`, `wfdrive-<n>.console.txt`) and the response says where each
one went. An `out` parameter is accepted and ignored on purpose: letting a
request name a file the server writes is a path-injection question that a tool
like this never needed to have.

To dump the DOM, ask for it:

```
curl 'http://127.0.0.1:9224/eval?expr=document.documentElement.outerHTML'
```

## BiDi commands used

`session.new` (with retry on session contention), `session.subscribe`,
`session.end`, `browsingContext.create`, `browsingContext.navigate`,
`browsingContext.captureScreenshot`, `browsingContext.getTree`,
`script.evaluate`, and the `log.entryAdded` event.

`network.*`, `input.*` and `storage.*` are not implemented.

## Use it as a library

The driver lives in `github.com/0magnet/wfdrive/bidi`, so a Go program can
embed it instead of shelling out. `skywire cli hv` does exactly that.

```go
d, err := bidi.Connect(ctx, "9223")
if err != nil { return err }
defer d.End() // MUST run on every exit path, signals included

title, err := d.Eval("document.title")
```

`bidi.Serve(ctx, bidiPort, ctrlAddr, announce)` is this command's serve mode,
so a host program can offer the same persistent control port under its own
name.

`Connect` waits out a lagging prior teardown, and every operation goes through
`WithTab`, which repairs a closed tab or a dropped socket in place. `End`
releases the session — skip it and the next client waits for a browser
restart.

## Alternatives

[`hupe1980/gowebdriver`](https://github.com/hupe1980/gowebdriver) is the closest
Go library. As of this writing it cannot open a BiDi session on its own — it
obtains BiDi only as a bolt-on to a WebDriver Classic session created by
chromedriver, and implements no `session.new`, `script.evaluate` or
`browsingContext.getTree`. Its only concrete driver is ChromeDriver, so it does
not target Gecko at all.

Extracted from [skywire](https://github.com/skycoin/skywire), where it drives
the browser-side test rig.
