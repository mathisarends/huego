# huego

A Philips Hue CLI backed by the bridge's local CLIP v2 REST API — no cloud
account, no Hue app, no cookie scraping. Interactive setup uses Philips's
cloud discovery endpoint once; all bridge control remains local.

## Install

Download the archive for your OS/arch from the
[latest release](https://github.com/mathisarends/huego/releases/latest),
extract it, and put `huego` (or `huego.exe`) on your `PATH`.

Or build from source:

```sh
go install github.com/mathisarends/huego@latest
```

## Setup

`huego setup` walks you through getting an application key from the bridge —
that's the credential huego needs for every other command:

```sh
huego setup
```

1. **Discovery.** It queries Philips's cloud endpoint
   `discovery.meethue.com` for bridges on your network. This does not require
   a cloud account. If it finds more than one bridge, huego lists them and
   asks you to pick; subsequent control requests go directly to that bridge.
2. **Link-button press.** It prints `Press the link button on your Hue
   Bridge, then hit Enter.` — the physical button on top of the bridge. huego
   then polls the bridge every 2 seconds for up to 60 seconds; the bridge
   itself only grants the request within a shorter window after the button
   press, which is how Hue proves the request came from someone with
   physical access, not a random device on the network.
3. **Registration.** Once you hit Enter, it registers the app and receives
   an application key (and a client key, reserved for future entertainment-
   streaming support — the bridge only ever returns it once, on this call).
4. **Storage.** Both are written to your OS config directory
   (`~/.config/huego` on Linux/macOS, `%AppData%\huego` on Windows), so every
   later command just works — no flags, no re-auth.

If setup can't run interactively (CI, containers, a headless box), skip it
and set credentials directly instead: `HUEGO_BRIDGE_IP`, `HUEGO_APP_KEY`,
`HUEGO_CLIENT_KEY` as env vars or in a `.env` file (see `.env.example`). You
still need to press the link button once to obtain the app key in the first
place — there's no way around that, it's how the bridge enforces physical
access for pairing.

## Commands

| Group   | Commands                                                                    |
| ------- | ---------------------------------------------------------------------------- |
| `setup` | discover a bridge and register an application key                          |
| `light` | `list`, `on`, `off`, `toggle`, `brightness`, `color`, `temperature`, `identify` |
| `room`  | same vocabulary as `light`, applied to a room's `grouped_light` service     |
| `zone`  | same vocabulary as `light`, applied to a zone's `grouped_light` service     |
| `scene` | `list`, `activate`                                                          |

Run `huego <command> --help` for flags on any command. The global `--json`
flag is available on every command and switches to newline-delimited JSON
output.

### `light` / `room` / `zone` — control vocabulary

`light`, `room`, and `zone` share one command surface. A room or zone
switches every light it contains in a single bridge call, via its
`grouped_light` service, instead of light by light.

| Command                            | Args              | Flags                              | Notes                                                          |
| ----------------------------------- | ----------------- | ----------------------------------- | ---------------------------------------------------------------- |
| `<group> list`                     | —                 | —                                   | Lists resources with their on/off state.                       |
| `<group> on <target>`              | UUID or name       | `--brightness/-b`, `--transition`   | Turns the target on.                                            |
| `<group> off <target>`             | UUID or name       | `--transition`                     | Turns the target off.                                          |
| `<group> toggle <target>`          | UUID or name       | `--transition`                     | Flips current on/off state.                                    |
| `<group> brightness <target> <pct>` | UUID/name, 0-100  | `--transition`                     | `0` turns the target off instead of dimming to nothing.        |
| `<group> color <target> <color>`   | UUID/name, color name, `#rgb`, or `#rrggbb` | `--brightness/-b`, `--transition` | Converts sRGB to the bridge's CIE xy color space.               |
| `<group> temperature <target> <kelvin>` | UUID/name, 2000-6536 K | `--brightness/-b`, `--transition`  | Converts kelvin to the bridge's supported mirek range (153-500); out-of-range values are rejected. |
| `<group> identify <target>`        | UUID or name       | —                                   | Makes the target breathe so you can spot it physically.         |

`--transition` is a fade duration in seconds. `<target>` accepts a resource
UUID or a complete name. Name matching ignores capitalization and surrounding
whitespace, but does not guess misspellings or abbreviated names. Unknown names
produce an error containing the known names for that resource type.

### `scene` — activate a stored scene

| Command                        | Args         | Flags                                            | Notes                                          |
| -------------------------------- | ------------ | -------------------------------------------------- | ------------------------------------------------- |
| `scene list`                   | —            | —                                                  | Lists scenes with their active/inactive status. |
| `scene activate <target>`      | UUID or name | `--brightness/-b`, `--transition`, `--dynamic`     | `--dynamic` starts the scene's dynamic palette instead of a static recall. |

```sh
huego light list
huego light on "Desk Lamp" --brightness 60
huego light color "Desk Lamp" "#ff8800"
huego light temperature "Desk Lamp" 2700
huego light off "Desk Lamp" --transition 2

huego room list
huego room toggle Kitchen

huego zone on "Living Room" --brightness 40

huego scene list
huego scene activate "Relax" --brightness 50 --transition 3
```

## API reference (`internal/hue`)

`internal/hue` is usable on its own as a small Go client for the Hue bridge —
everything the CLI does is a thin wrapper over it.

```go
client := hue.NewClient(config.Bridge{IP: "192.168.1.10", AppKey: "..."})

lights, err := client.Lights.List(ctx)
light, err  := client.Lights.FindByName(ctx, "Desk Lamp")

_, err = hue.TurnOn(ctx, client.Lights, light.ID, nil, nil, nil)
_, err = hue.SetHex(ctx, client.Rooms, roomID, "#ff8800", nil, nil)
_, err = client.Scenes.Activate(ctx, sceneID, hue.SceneActivation{Dynamic: true})
```

| Type / function | Signature | Notes |
| ---------------- | --------- | ------- |
| `hue.NewClient` | `func(bridge config.Bridge) *Client` | Skips TLS verification — the bridge presents a self-signed certificate. |
| `Client.Lights` | `*Lights` | `List`, `GetOne`, `FindByName`, `ResolveTarget` over `light` resources. |
| `Client.Rooms` / `Client.Zones` | `*Rooms` / `*Zones` | Same namespace methods over `room`/`zone`; light commands act through the group's `grouped_light` service. |
| `Client.Scenes` | `*Scenes` | `List`, `GetOne`, `FindByName`, plus `Recall` and `Activate`. |
| `hue.TurnOn` / `TurnOff` / `Toggle` | `func(ctx, Controllable, id string, ...) (APIResponse[ResourceIdentifier], error)` | Shared light-command functions; work on any `Controllable` (lights, rooms, zones). |
| `hue.SetBrightness` | `func(ctx, Controllable, id string, brightness float64, transition *float64) (...)` | `brightness == 0` turns the target off instead of dimming to zero. |
| `hue.SetRGB` / `SetHex` | `func(ctx, Controllable, id string, ..., brightness, transition *float64) (...)` | Converts sRGB → CIE xy internally via `hue.RGBToXY`. |
| `hue.SetColorTemperature` | `func(ctx, Controllable, id string, kelvin int, brightness, transition *float64) (...)` | Converts kelvin → mirek via `hue.KelvinToMirek`; out-of-range values return an error. |
| `hue.Brighten` / `Dim` | `func(ctx, Controllable, id string, by float64) (...)` | Relative brightness steps via the bridge's `dimming_delta`. |
| `hue.Identify` | `func(ctx, Controllable, id string) (...)` | Triggers the `breathe` alert effect. |
| `hue.ParseColor` | `func(color string) (RGB, error)` | Accepts hex (`#rgb`, `#rrggbb`) or a built-in color name (`red`, `warm_white`, ...). |
| `hue.RGBToXY` / `hue.XYToRGB` | `func(RGB) ColorXY` / `func(ColorXY, brightness float64) RGB` | sRGB ⇄ CIE xy, gamma-corrected per the Hue color spec. |
| `hue.KelvinToMirek` / `hue.MirekToKelvin` | `func(int) (int, error)` | Color temperature conversion; kelvin must be within `[KelvinMinimum, KelvinMaximum]` (2000-6536 K). |
| `hue.DiscoverBridges` | `func(ctx) ([]DiscoveredBridge, error)` | Queries `discovery.meethue.com`. |
| `hue.RegisterAppKey` | `func(ctx, bridgeIP string) (RegisteredApp, error)` | Polls the bridge's `/api` endpoint every 2s for up to 60s until the link button is pressed. |

Every `Controllable`-accepting function works identically whether you pass
`client.Lights`, `client.Rooms`, or `client.Zones` —
that's the same polymorphism the CLI's `light`/`room`/`zone` commands lean on.

## Development

```sh
go build ./...
go test ./...
go vet ./...
```

```sh
./scripts/format.sh          # gofumpt + gci
./scripts/lint.sh            # golangci-lint
./scripts/check-coverage.sh  # go test -cover, threshold defaults to 90
```

## See also

[hueify](https://github.com/mathisarends/hueify), a Python client covering
the same lighting control plus entertainment-area DTLS streaming and the
bridge's SSE event stream, for anyone who needs those.
