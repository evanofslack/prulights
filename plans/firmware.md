# Frame firmware: what it reads and how it behaves

The server and the emulator (`/emulator?mode=device`) already speak this. Firmware is the last phase
(see `prulights.md`); this is the contract it has to meet.

## Preparing a frame

1. **Identity, on the server.** Either `/admin` → Frames → "Prepare a frame" (downloads
   `frame-<id>.json`), or `docker compose exec prulights prulights frame new -out frame.json`. Both mark
   the frame for a new person by default (`-invite=false` to skip): for 30 days, whoever scans its
   code signed out makes an account and gets the frame in one step.
2. **Wi-Fi, locally.** `just frame-config -from frame.json -wifi "Home:password" -wifi "Cape:password"
   -out sd`. The networks go only into these files, never to the server.
3. Copy `sd/` onto the frame's SD card (or write `config.json` into NVS when flashing).

`sd/` holds:

| File | What |
|---|---|
| `config.json` | identity + networks (below). Holds the device token: keep it private. |
| `setup.bin.gz` | the welcome screen (white tower, QR code to `/claim?code=`), for a first boot with no network |
| `wifi.bin.gz` | the Wi-Fi setup screen (hotspot QR code, name, password, `192.168.4.1`) |
| `*.png` | previews of the two, for you |

## config.json (version 1)

```json
{
  "version": 1,
  "server": "https://prulights.evanslack.dev",
  "device_id": 7,
  "token": "…",
  "claim_code": "K7Q-9MX",
  "ap": { "ssid": "prulights-K7Q9", "password": "mxq4ad7tph" },
  "wifi": [{ "ssid": "Home", "password": "…" }, { "ssid": "Cape", "password": "…" }]
}
```

- `token` goes in `Authorization: Bearer` on every `/api/device/v1` call. A 401 means the server
  forgot the frame (an admin removed it): register again (`POST /register`, sending `ap`), then show
  the new welcome screen.
- `ap` is the frame's own setup hotspot. Send it on `POST /register` and `POST /events`
  (`ap_ssid`, `ap_password`) if it ever changes, so the server's Wi-Fi screen matches.
- `wifi` is up to 5 networks. Networks added on the setup page are appended (dropping the oldest).

## Each wake

1. Scan; join the strongest network in `wifi`.
2. If none answer after 3 tries: show the saved `wifi.bin.gz`, open the hotspot (below) for 10
   minutes, then sleep. The screen keeps saying how to set up Wi-Fi without using power; the next
   wake or a button hold tries again.
3. Online: `POST /events` with the queued events, `battery_mv`, `power` and `firmware`. The answer is
   the state: `bundle_version`, `next_checkin_s`, `server_time` (set the clock), `settings`.
4. If `bundle_version` changed: `GET /bundle?have=<old>`, download the pictures whose `sha256` it
   doesn't have (`GET` each `url`, check the hash of the unzipped bytes), keep the bundle on flash or
   SD.
5. Show the right picture, redrawing only when it changed:
   - unclaimed: the `setup` item for tonight
   - in quiet hours: `goodnight` for tonight
   - else the current view for tonight (the default after a sync or rollover)
   - past the last night: the `stale` item
   - no network and nothing saved: `wifi`

   A frame set to "dark at night" (`settings.theme` `auto`) gets each night's views twice: `theme`
   `dark`, and `theme` `light` with `from` and `until` (sunrise to lights-on). Of the items for a
   night and view, show the one whose window holds the time, else the one without a window. Redraw
   when the window opens or closes: wake at `from` and `until` as well as at the 4am rollover.
6. Sleep `next_checkin_s`, or until the 4am rollover (or a light picture's `from`/`until`),
   whichever is first.

"Tonight" rolls over at 04:00 America/New_York.

## The button

| Gesture | Does |
|---|---|
| Short | Next view from the bundle, no Wi-Fi. Queue `short`. |
| Long | Instant feedback, queue `long` + `seen` (with tonight's date), then sync. |
| Double (400 ms window) | Queue `double`, sync now. |
| Hold ~8 s | Open the Wi-Fi setup hotspot and show `wifi.bin.gz`. |

## Setup hotspot

- WPA2 network `ap.ssid` / `ap.password`. The Wi-Fi screen's QR code is
  `WIFI:T:WPA;S:<ssid>;P:<password>;;`, which phone cameras join directly.
- Captive-portal DNS answers every name with `192.168.4.1`, so phones pop the page open. Typing
  `http://192.168.4.1` works too.
- The page lists nearby networks, takes a password, adds the network to `wifi`, then tries it. On
  success it says "Connected — the frame is updating" and closes the hotspot. The frame syncs, and if
  it's unclaimed it shows the welcome screen: step 2, scan to connect your phone.
- The phone can't make a passkey while on the hotspot (no internet), which is why Wi-Fi and the
  account are two steps.
