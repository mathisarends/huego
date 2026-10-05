# Using huego from an agent

Use `--json`, inspect the exit code, and prefer resource UUIDs for writes.
All control requests use the selected bridge's local HTTPS API. Pairing requires
one physical link-button press. No real bridge is needed for offline discovery.

## Discover schemas and examples

```sh
huego resource types --json
huego resource schema scene --json
huego resource example scene --json
huego resource example light --method PUT --json
huego resource list --json
huego resource get light <light-uuid> --json
```

The embedded catalog combines 41 types from OpenHue revision
`1ffc817857abf456d5ff2ae50400ef768dbce28e` with a separately attributed supplement.
It now includes `switch_input_configuration`, automation creation/deletion, and
entertainment-area creation/deletion and locations. `additional_sources` reports
the supplement's references; `notes` records remaining limitations. The catalog
is an API inventory, not a promise that every bridge supports every operation.

Schemas preserve descriptions, examples, required properties, bounds and enums.
Local references are expanded. Hue fields and enum values named `on` remain
strings, while actual JSON booleans remain booleans. Read-only annotations are
information for authors; the schema validator does not strip fields.

`resource example` emits editable JSON for common tasks. Replace placeholder
UUIDs. An automation example's empty configuration must be filled using its
behavior script's `configuration_schema`; it is not a ready-to-enable routine.

## JSON input, output and errors

Provide exactly one of `--data` and `--file` for native create/update/ensure.
`--file -` reads stdin. Input is a JSON object, limited to 4 MiB; UTF-8 BOMs are
accepted. Unknown fields, explicit null/false/zero and large integers survive
native JSON round trips. In PowerShell, prefer files or stdin:

```powershell
Get-Content -Raw .\scene.json | huego scene create --file - --dry-run --json
```

Native reads and writes emit one `{"errors":[],"data":[]}` envelope on stdout.
Read resources use `id`/`type`; write identifiers use `rid`/`rtype`.
`light list`, `room list`, `zone list` and `scene list` keep summary arrays;
scene summaries also include the group reference. Use `resource list <type>` for
complete native data. Workflow no-ops report `changed:false`.

All failures exit nonzero. With `--json`, stderr contains a structured
`{"error":{"code":"...","message":"...","details":...}}` object, including
argument, configuration, HTTP and target-resolution failures. Error codes include
`ambiguous_target`, `target_not_found`, `validation_failed`, `reference_not_found`,
`unsupported_operation`, `unsupported_capability`, `not_configured`,
`bridge_http_error`, `bridge_error`, `bridge_unreachable`, `timeout` and
`workflow_failed`. Candidate IDs and HTTP statuses are structured details.
Native bridge error envelopes, including partial successes, remain on stdout.
Check both streams and the exit code; never blindly retry an uncertain POST.

## Validation and previews

```sh
huego resource validate scene --file scene.json --json
huego resource validate light --method PUT --file light.json --json
huego scene create --file scene.json --dry-run --json
huego scene create --file scene.json --check --dry-run --json
huego scene create --file scene.json --check --json
huego light on <light-uuid> --brightness 60 --dry-run --json
huego scene activate Relax --room Office --dry-run --json
```

Native dry runs validate against the embedded schema by default. `--validate`
also enables offline schema validation for native execution. `--check` implies
schema validation and reads the bridge inventory: it checks local references,
scene membership, supported light features/temperature ranges, smart-scene group
relationships, and script-specific configuration/trigger schemas. Public image
references are not checked against the local inventory. The bridge remains the
final authority for firmware semantics, limits and hardware compatibility.

Plans identify exactly what was checked using `validation.schema`,
`validation.references` and `validation.capabilities`. `not_checked` is explicit.
Offline UUID/create previews need no credentials; names, group control, toggles,
snapshots and membership changes require reads. A preview never writes.
Lighting controls, scene recalls, native mutations and workflow mutations all
support `--dry-run`. `--check` is for native/workflow writes; it does not add
hardware validation to the original on/off/color convenience commands.

`--raw` on the generic `resource` commands bypasses the operation catalog and
schema checks for firmware additions. It retains JSON-object/path validation:

```sh
huego resource update future_service <uuid> --raw --file update.json --dry-run --json
```

## Rooms, zones and individual scene actions

```sh
huego room add Office "Desk Lamp" "Ceiling Lamp" --dry-run --json
huego room add Office "Desk Lamp" --json
huego room add Office "Motion Sensor" --devices --json
huego room remove Office "Desk Lamp" --json
huego room move "Desk Lamp" Study --dry-run --json
huego zone add Upstairs "Desk Lamp" --json
huego zone remove Upstairs "Desk Lamp" --json

huego scene list --room Office --json
huego scene activate Relax --room Office --json
huego scene get Relax --zone Upstairs --json
huego scene snapshot "Working" --room Office --ensure --json
huego scene clone Relax "Evening copy" --room Office --json
huego scene set-light Relax "Desk Lamp" --room Office --brightness 35 --temperature 2700 --json
huego scene set-light Relax "Desk Lamp" --room Office --off --json
huego scene remove-light Relax "Desk Lamp" --room Office --json
```

Room membership converts light references to owning devices; `--devices` permits
accessories. Zone membership uses light services. Add/remove operations preserve
other members and do not write when nothing changed. `room move` moves the whole
owning device, including any other light services of that device.

Scenes and smart scenes accept `--room` or `--zone` for list/get/update/delete,
rename and helper actions. Matching remains exact and case-insensitive; duplicate
matches within the selected scope fail. UUID targets are also checked against an
explicit scope. Snapshot captures current member lights, removes read-only state
fields, and chooses color or valid color temperature. Clone retains the source
group. `set-light` merges one action and preserves other actions and unknown
fields. Supply either lighting flags or native action JSON with `--data/--file`.

Membership and single-action edits re-read the edited array before writing and
reject detected concurrent changes. Hue offers no atomic compare-and-swap here:
a change between that final read and PUT remains possible. Moves are sequential,
not transactions; errors include completed responses and remaining requests.
Re-running a move re-plans against current membership.

## Repeatable creation and automation

```sh
huego room ensure Office --file office.json --json
huego scene ensure Focus --file focus.json --json
huego smart-scene ensure Workday --file workday.json --json
huego behavior-script list --json
huego behavior-script get <script-uuid> --json
huego resource example behavior_instance --json
huego behavior-instance create --file automation.json --check --dry-run --json
huego behavior-instance ensure "Morning routine" --file automation.json --check --json
huego behavior-instance enable "Morning routine" --json
huego behavior-instance disable "Morning routine" --json
huego behavior-instance delete "Morning routine" --json
```

`ensure` creates a missing resource, updates a unique existing name, and reuses
its ID. Scenes/smart scenes are matched by name and the payload's group. The name
argument sets `metadata.name`. Payloads must be suitable for creation, including
required fields. Unchanged values produce a no-op. An existing automation cannot
silently switch to another script. Duplicate matches fail. Concurrent independent
creators can still race; after a timeout, re-read the bridge before retrying.

Native updates remain partial PUTs. Arrays such as `children`, `actions` and
`week_timeslots` replace that field; use the helpers for individual member/action
changes. Unknown configuration fields are not discarded. Native JSON uses CIE xy,
mirek, and milliseconds; convenience flags use colors, kelvin, and seconds.

## Observe completion and events

```sh
huego resource wait light "Desk Lamp" --path /on/on --equals true --timeout 10s --json
huego watch --type motion --count 1 --timeout 30s --json
huego watch --id <resource-uuid> --timeout 2m --json
```

`wait` supports JSON pointers, including array indices. `--equals` is a JSON
value (strings must include JSON quotes); default polling interval is 1s and
default timeout is 30s. Missing fields do not satisfy a wait. Timeouts fail with
nonzero status. `watch` emits one JSON object per SSE message and supports type/ID
filters, bounded duration/count, cancellation, and transient reconnects with
`Last-Event-ID`. Without bounds it runs until interrupted. A reconnect does not
guarantee the bridge retains every missed event; re-read state when needed.

## Bridge profiles and noninteractive pairing

```sh
huego profile discover --json
huego setup --ip 192.168.1.10 --non-interactive --bridge downstairs --json
huego profile save upstairs --file bridge-credentials.json --json
huego profile list --json
huego --bridge downstairs room list --json
```

Press the physical link button before noninteractive pairing. Supplying `--ip`
skips cloud discovery. Noninteractive setup reports only address/profile, not
credentials. Interactive setup fails promptly on closed stdin.

`profile save` accepts `bridge_ip`, `app_key`, optional `client_key`, and replaces
that profile. Profile names are validated; profile lists omit credentials.
Explicit `--bridge` overrides `HUEGO_PROFILE`. A selected profile isolates its
credentials from bridge/key environment overrides. Without a profile, existing
environment / `.env` / default config behavior remains available.

## Entertainment and firmware boundaries

```sh
huego resource example entertainment_configuration --json
huego entertainment-configuration create --file area.json --check --json
huego entertainment-configuration update TV --file positions.json --json
huego entertainment-configuration stream TV --file frames.json --fps 25 --dry-run --json
huego entertainment-configuration stream TV --file frames.json --fps 25 --json
huego entertainment-configuration delete TV --json
```

Area locations reference **entertainment services**, not light UUIDs. Retrieve the
area to discover channel IDs. A finite frame file uses normalized RGB components:

```json
{"frames":[{"0":[1,0,0],"1":[0,0,1]},{"0":[0,1,0],"1":[1,0,0]}]}
```

Streaming requires the client key from the same pairing as the app key. It checks
channels and frame values before activation, uses DTLS 1.2 on UDP 2100 at 1-60 FPS,
and attempts to release the area on completion/failure/cancellation. It refuses
to take over an already-active area. A stream preview validates RGB values but
reports channels as unchecked. This is frame playback, not screen/audio capture.

Wall switches have native `switch-input-configuration get/update` and schema
support. MotionAware configuration supports reading, enable/disable and naming;
full Bridge Pro MotionAware setup still requires firmware-specific payloads via
`--raw`. Cloud-account functions and Hue Secure video/subscription workflows are
not exposed by this local bridge CLI. Consult the current official Hue reference
for those boundaries rather than interpreting a listed resource as app parity.

Automated tests use fake HTTPS bridges, SSE frames, DTLS transport substitutes
and a real local DTLS PSK handshake.
They do not certify compatibility with physical lights, Bridge Pro firmware or
Hue app/cloud features. Hardware validation remains necessary.
