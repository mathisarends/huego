# huego

A Philips Hue CLI backed by the bridge's local CLIP v2 REST API — no cloud
account, no Hue app. Setup may ask Philips's discovery service once where the
bridge is; everything after that talks to the bridge directly.

huego is a single static binary that starts in milliseconds, so it suits
keybindings, status bars, scripts, and AI agents alike: every command has
`--json` output, failures have distinct exit codes and JSON error objects,
and live changes stream from the bridge's event stream.

## Install

Download the archive for your OS/arch from the
[latest release](https://github.com/mathisarends/huego/releases/latest),
extract it, and put `huego` (or `huego.exe`) on your `PATH`. Or build from
source:

```sh
go install github.com/mathisarends/huego@latest
```

## Setup

```sh
huego setup                      # find the bridge via discovery.meethue.com
huego setup --ip 192.168.1.20    # or skip discovery
```

Press the link button on the bridge when asked; setup waits up to 60 seconds
for it, then saves the bridge address and application key to
`~/.config/huego/bridge.json` (`%AppData%\huego` on Windows). Progress goes to
stderr and the result to stdout, so `huego setup --ip … --json` can be
scripted.

`HUEGO_BRIDGE_IP`, `HUEGO_APP_KEY`, and `HUEGO_CLIENT_KEY` — as environment
variables or in a `.env` file in the working directory (see `.env.example`) —
override the saved values. `HUEGO_CONFIG_DIR` moves the config directory.

## Commands

| Command | Subcommands |
| ------- | ----------- |
| `light` | `list`, `get`, `effect`, and the control vocabulary below |
| `room`, `zone` | `list`, `get`, and the control vocabulary, applied through the group's `grouped_light` in one bridge call |
| `home` | `get` and the control vocabulary for every light on the bridge |
| `scene` | `list [--group <room or zone>]`, `get`, `activate [-b] [-t] [--dynamic]` |
| `smart-scene` | `list [--group …]`, `get`, `activate`, `deactivate` — scenes that follow the time of day |
| `automation` | `list`, `get`, `enable`, `disable` — wake-ups, timers, and other Hue automations |
| `device` | `list`, `get` — product, model, firmware, connectivity, battery, services |
| `sensor` | `list [--type …]`, `enable <device>`, `disable <device>` |
| `bridge` | `get` — bridge ID, address, model, firmware, time zone |
| `events` | stream live changes, `--type` to filter |
| `api` | raw CLIP v2 request for anything without a dedicated command |
| `setup` | find a bridge and register huego with it |
| `completion` | shell completion script for bash, zsh, fish, or PowerShell |

### Control vocabulary

`light`, `room`, `zone`, and `home` share these commands. `home` takes no
`<target>`.

| Command | Flags | Notes |
| ------- | ----- | ----- |
| `on <target>` | `-b`, `-t` | |
| `off <target>` | `-t` | |
| `toggle <target>` | `-t` | |
| `brightness <target> <0-100>` (`bri`) | `-t` | `0` turns off |
| `brighten <target> [step]` (`up`) | | default step 10, turns on |
| `dim <target> [step]` (`down`) | | default step 10, stops at the minimum |
| `color <target> <color>` (`colour`) | `-b`, `-t` | name (`warm_white`, `red`, …), `#rgb`, or `#rrggbb` |
| `temperature <target> <kelvin>` (`temp`, `ct`) | `-b`, `-t` | 2000–6536 K, limited further by the light |
| `identify <target>` | | makes the lights breathe |

`-b/--brightness` is a percentage, `-t/--transition` a fade in seconds.
`<target>` is a UUID or a full name; matching ignores case and surrounding
spaces but never guesses. Scenes and smart scenes also accept
`<room or zone>/<name>`, since every room tends to have a "Relax". Shell
completion fills in names from the bridge, for arguments and for `--group`.

```sh
huego light on "Desk Lamp" -b 60
huego light color "Desk Lamp" "#ff8800" -t 2
huego light effect "Desk Lamp" candle
huego room dim Kitchen 20
huego home off -t 3
huego scene activate Office/Relax --dynamic
huego sensor list --type temperature
huego sensor disable "Hallway sensor"
huego api get light/<id>
huego smart-scene activate "Office/Natural light"
huego automation disable "Weekday wake-up"
huego api get behavior_instance/<id>
```

## Scripts and agents

- `--json` works on every command. Lists are JSON arrays (empty, never null),
  `get` commands single objects, control commands the bridge's response, and
  `events --json` one JSON object per line.
- `list --names` prints one name per line, for `fzf` or `walker --dmenu`.
- Errors go to stderr; with `--json` as
  `{"error": "…", "code": "…", "candidates": […]}`. `candidates` lists the
  IDs to choose from when a name is ambiguous.

| Exit code | `code` | Meaning |
| --------- | ------ | ------- |
| 0 | | success |
| 1 | `error`, `bridge_error` | the bridge rejected the request, or another failure |
| 2 | `usage` | invalid arguments or flags; nothing was sent to the bridge |
| 3 | `not_found`, `ambiguous` | no resource, or more than one, matches the name |
| 4 | `not_configured`, `unauthorized` | run `huego setup`, or the app key was revoked |
| 5 | `unreachable` | the bridge did not answer |

## Library

`internal/hue` is the client the CLI wraps:

```go
client := hue.NewClient(config.Bridge{IP: "192.168.1.10", AppKey: "..."})

lights, err := client.Lights.ListWithRooms(ctx)
id, err := client.Rooms.ResolveTarget(ctx, "Kitchen")
_, err = hue.SetHex(ctx, client.Rooms, id, "#ff8800", nil, nil)
_, err = client.Scenes.Activate(ctx, sceneID, hue.SceneActivation{Dynamic: true})
devices, err := client.Devices.ListDetailed(ctx)

err = client.WatchEvents(ctx, hue.EventWatch{Event: func(e hue.Event) { … }})
body, err := client.Raw(ctx, http.MethodGet, "behavior_instance", nil)
```

`Client.Lights`, `Rooms`, `Zones`, and `Home` all satisfy `hue.Controllable`,
so `hue.TurnOn`, `SetBrightness`, `Brighten`, `SetHex`,
`SetColorTemperature`, `Identify`, and friends work on any of them.

## Development

```sh
go build ./...
go test ./...
./scripts/format.sh          # gofumpt + gci
./scripts/lint.sh            # golangci-lint
./scripts/check-coverage.sh  # go test -cover, threshold defaults to 90
```

## See also

[hueify](https://github.com/mathisarends/hueify), the Python client huego
was ported from, which also covers entertainment-area streaming.
