# huego

A Philips Hue CLI backed by the bridge's local CLIP v2 REST API — no cloud
account, no Hue app. Ported from the [mathisarends/hueify](https://github.com/mathisarends/hueify)
Python client; project layout modeled after [mathisarends/spoti](https://github.com/mathisarends/spoti).

## Install

```sh
go install github.com/mathisarends/huego@latest
```

Or grab a prebuilt binary from [Releases](https://github.com/mathisarends/huego/releases).

## Setup

```sh
huego setup
```

Discovers your bridge on the network, waits for you to press its link
button, then registers an application key and stores it in your OS config
directory. You can also set `HUEGO_BRIDGE_IP` / `HUEGO_APP_KEY` directly
(see `.env.example`).

## Usage

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

`light`, `room`, and `zone` all share the same command vocabulary
(`on`, `off`, `toggle`, `brightness`, `color`, `temperature`, `identify`).
Rooms and zones control every light in the group in one bridge call via
their `grouped_light` service. A `<target>` argument accepts either a
resource UUID or a name — names are matched fuzzily if there's no exact
match.

Add `--json` to any command for machine-readable, newline-delimited JSON
output.

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
