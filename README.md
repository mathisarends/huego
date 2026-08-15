# huego

A Philips Hue CLI backed by the bridge's local CLIP v2 REST API — no cloud
account, no Hue app, no cookie scraping. Ported from the
[mathisarends/hueify](https://github.com/mathisarends/hueify) Python client;
project layout modeled after [mathisarends/spoti](https://github.com/mathisarends/spoti).

## Install

Download the archive for your OS/arch from the
[latest release](https://github.com/mathisarends/huego/releases/latest),
extract it, and put `huego` (or `huego.exe`) on your `PATH`.

Or build from source:

```sh
go install github.com/mathisarends/huego@latest
```

## Setup

```sh
huego setup
```

Discovers your bridge on the network via `discovery.meethue.com`, waits for
you to press its link button, then registers an application key (and, for
future entertainment-streaming support, a client key) and stores them in
your OS config directory (`~/.config/huego` on Linux/macOS,
`%AppData%\huego` on Windows).

Credentials can also come from env vars instead of `setup`: `HUEGO_BRIDGE_IP`,
`HUEGO_APP_KEY`, `HUEGO_CLIENT_KEY` (see `.env.example`).

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
| `<group> color <target> <hex>`     | UUID/name, `#rgb` or `#rrggbb` | `--brightness/-b`, `--transition` | Converts sRGB to the bridge's CIE xy color space.               |
| `<group> temperature <target> <kelvin>` | UUID/name, kelvin | `--brightness/-b`, `--transition`  | Converts kelvin to mirek, clamped to the bridge's supported range (153-500 mirek). |
| `<group> identify <target>`        | UUID or name       | —                                   | Makes the target breathe so you can spot it physically.         |

`--transition` is a fade duration in seconds. `<target>` accepts a resource
UUID or a name; names are matched exactly first, then fuzzily (closest match
by edit distance) if there's no exact hit.

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

lights, err := client.LightNamespace.List(ctx)
light, err  := client.LightNamespace.FindByName(ctx, "Desk Lamp")

_, err = hue.TurnOn(ctx, client.LightNamespace, light.ID, nil, nil, nil)
_, err = hue.SetHex(ctx, client.RoomNamespace, roomID, "#ff8800", nil, nil)
_, err = client.SceneNamespace.Activate(ctx, sceneID, hue.SceneActivation{Dynamic: true})
```

| Type / function | Signature | Notes |
| ---------------- | --------- | ------- |
| `hue.NewClient` | `func(bridge config.Bridge) *Client` | Skips TLS verification — the bridge presents a self-signed certificate. |
| `Client.LightNamespace` | `*LightNamespace` | `List`, `GetOne`, `FindByName`, `ResolveTarget` over `light` resources. |
| `Client.RoomNamespace` / `Client.ZoneNamespace` | `*RoomNamespace` / `*ZoneNamespace` | Same namespace methods over `room`/`zone`; light commands act through the group's `grouped_light` service. |
| `Client.SceneNamespace` | `*SceneNamespace` | `List`, `GetOne`, `FindByName`, plus `Recall` and `Activate`. |
| `hue.TurnOn` / `TurnOff` / `Toggle` | `func(ctx, Controllable, id string, ...) (APIResponse[ResourceIdentifier], error)` | Shared light-command functions; work on any `Controllable` (lights, rooms, zones). |
| `hue.SetBrightness` | `func(ctx, Controllable, id string, brightness float64, transition *float64) (...)` | `brightness == 0` turns the target off instead of dimming to zero. |
| `hue.SetRGB` / `SetHex` | `func(ctx, Controllable, id string, ..., brightness, transition *float64) (...)` | Converts sRGB → CIE xy internally via `hue.RGBToXY`. |
| `hue.SetColorTemperature` | `func(ctx, Controllable, id string, kelvin int, brightness, transition *float64) (...)` | Converts kelvin → mirek via `hue.KelvinToMirek`, clamped to `[MirekMinimum, MirekMaximum]`. |
| `hue.Brighten` / `Dim` | `func(ctx, Controllable, id string, by float64) (...)` | Relative brightness steps via the bridge's `dimming_delta`. |
| `hue.Identify` | `func(ctx, Controllable, id string) (...)` | Triggers the `breathe` alert effect. |
| `hue.ParseColor` | `func(color string) (RGB, error)` | Accepts hex (`#rgb`, `#rrggbb`) or a built-in color name (`red`, `warm_white`, ...). |
| `hue.RGBToXY` / `hue.XYToRGB` | `func(RGB) ColorXY` / `func(ColorXY, brightness float64) RGB` | sRGB ⇄ CIE xy, gamma-corrected per the Hue color spec. |
| `hue.KelvinToMirek` / `hue.MirekToKelvin` | `func(int) (int, error)` | Color temperature conversion; kelvin conversion clamps to `[153, 500]` mirek. |
| `hue.DiscoverBridges` | `func(ctx) ([]DiscoveredBridge, error)` | Queries `discovery.meethue.com`. |
| `hue.RegisterAppKey` | `func(ctx, bridgeIP string) (RegisteredApp, error)` | Polls the bridge's `/api` endpoint every 2s for up to 60s until the link button is pressed. |

Every `Controllable`-accepting function works identically whether you pass
`client.LightNamespace`, `client.RoomNamespace`, or `client.ZoneNamespace` —
that's the same polymorphism the CLI's `light`/`room`/`zone` commands lean on.

## Scope

This port covers lighting control (lights, rooms, zones, scenes) and bridge
onboarding. hueify's entertainment-area DTLS streaming and SSE event stream
are not ported — high implementation complexity for little CLI value.

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
