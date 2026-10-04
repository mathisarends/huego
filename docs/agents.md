# Using huego from an agent

Use `--json`, check the process exit code, and use resource UUIDs for writes.
Pair a bridge once with `huego setup`, or provide `HUEGO_BRIDGE_IP` and
`HUEGO_APP_KEY`. Configuration follows the existing environment / `.env` / OS
config-file precedence. All control requests use the bridge's local HTTPS API.

## Discover resources and request shapes

```sh
huego resource types --json
huego resource schema room --json
huego resource schema scene --json
huego resource schema smart_scene --json
huego resource list --json
huego resource list device --json
huego resource get light <light-uuid> --json
huego resource get device <owner-rid> --json
```

`types` returns source, revision and collection/item HTTP methods. `schema`
also returns `request_schemas.POST` and/or `request_schemas.PUT`, when documented.
Both work offline without bridge credentials. Local `$ref` links are expanded.
Schemas retain fields, types, required keys, enums, bounds and read/write markers;
upstream descriptions and examples are omitted. Read-only fields in a shared
upstream schema are guidance to exclude those fields from writes.

The catalog is pinned to OpenHue revision
[`1ffc817857abf456d5ff2ae50400ef768dbce28e`](https://github.com/openhue/openhue-api/tree/1ffc817857abf456d5ff2ae50400ef768dbce28e),
retrieved 2026-10-04. It documents API operations, not which services a particular
bridge exposes. Inspect the real inventory and each resource's capability fields.

Resources contain references such as `owner`, `children`, `services` and `group`.
Follow each reference's `rid` using its `rtype`. Room membership uses device
references; zones can group light services. Scene actions target light services,
and smart-scene timeslots target scenes in the same room or zone.

Complete reads and writes return one CLIP envelope on stdout:

```json
{"errors":[],"data":[{"rid":"01234567-89ab-cdef-0123-456789abcdef","rtype":"room"}]}
```

Read resources use `id` / `type`; write results use `rid` / `rtype`. Unknown fields,
large integer values and explicit nulls survive JSON round trips. The original
`light list`, `room list`, `zone list` and `scene list` return summary arrays;
`resource list <type>` returns complete native data for those types too.

Errors in a CLIP envelope remain in stdout and cause exit code 1, even when the
HTTP status is 200 or some `data` entries succeeded. HTTP, configuration,
validation and target-resolution failures produce diagnostics on stderr and a
nonzero exit code. Do not retry a partial creation blindly: inspect the bridge
to determine what succeeded. An empty item read returns an empty envelope and
fails; an empty collection read succeeds.

## Write JSON and inspect a request first

```sh
huego resource create room --file office.json --dry-run --json
huego resource create room --file office.json --json
huego resource update room <room-uuid> --file office-update.json --json
huego resource delete scene <scene-uuid> --dry-run --json
```

POST addresses a collection; PUT and DELETE address an item. PUT sends only the
supplied object; it does not merge a full snapshot into the request. A list such
as `children` or `actions` generally replaces that field, so inspect current
members before editing it. Use `get` afterward to verify the resulting state.

Provide exactly one of `--data/-d` and `--file/-f` for create/update. `--file -`
reads stdin. A UTF-8 BOM is accepted in file/stdin input, and payloads are limited
to 4 MiB. Use files or stdin in PowerShell to avoid native executable quoting
differences:

```powershell
Get-Content -Raw .\office.json | huego room create --file - --dry-run --json
huego room create --file .\office.json --json
```

In shells with POSIX quoting, inline JSON works too:

```sh
huego device update <device-uuid> --data '{"metadata":{"name":"Desk"}}' --json
```

All new create/update/delete commands and task commands support `--dry-run`.
The result includes `method`, the local CLIP `path`, and optional `body`, without
credentials. Creation and UUID targets work offline. A name requires a bridge
read for resolution, but a dry run never writes. The original lighting control
commands and `scene activate` retain their existing flags; they do not acquire
`--dry-run` in this change.

A target is a UUID or a complete, case-insensitive name. Duplicate matches fail
with matching IDs. Unnamed sensors must be addressed by UUID. Creation does not
deduplicate or upsert: list existing resources before creating one.

## Create a room, a scene and a daily schedule

The [JSON templates](../examples/requests) use placeholder UUIDs. Replace them
with IDs from your bridge before running the following commands.

1. Read `resource list device` and choose the devices for a room. Fill
   `examples/requests/room.json` with their IDs. Preview it with
   `huego room create --file room.json --dry-run --json`, then create the room
   and record its returned `data[0].rid`.
2. Read the devices' `services` to find their light IDs. Fill
   `examples/requests/scene.json` with the new room ID and those light IDs.
   Create the scene with `huego scene create --file scene.json --json` and
   record its returned ID. Scenes require metadata, group and actions.
3. Fill `examples/requests/smart-scene.json` with the same room ID and scenes
   belonging to it. Create the schedule with
   `huego smart-scene create --file smart-scene.json --json`. Activate the
   returned ID with `huego smart-scene activate <smart-scene-uuid> --json`.
4. Verify with `room get`, `scene get` and `smart-scene get`. To modify the
   schedule later, send its changed `week_timeslots` with `smart-scene update`.

`zone create/update/delete` uses the same native JSON surface as rooms. Supply
light/service references for its children. Configuration writes to `room update`
and `zone update` go to the room/zone endpoint. The existing `room on`, `zone color`
and other light controls still operate on the group's `grouped_light` service.

The light-control convenience flags use seconds and kelvin. Native JSON uses
the CLIP fields and units: durations in milliseconds, temperatures in mirek,
and colors as CIE xy. For example, 2700 K is approximately 370 mirek. The scene
template uses mirek for a warm white state.

## Devices, sensors, automations and effects

```sh
huego device rename <device-uuid> "Office lamp" --dry-run --json
huego device identify <device-uuid> --json
huego device usertest <device-uuid> true --json
huego motion enable <motion-uuid> --json
huego motion sensitivity <motion-uuid> 2 --json
huego temperature disable <temperature-uuid> --json
huego light effect <light-uuid> candle --json
huego light effect <light-uuid> no_effect --json
huego light powerup <light-uuid> last_on_state --json
huego zigbee-device-discovery search <discovery-uuid> --json
huego device-software-update install <update-uuid> --json
huego entertainment-configuration start <area-uuid> --json
huego entertainment-configuration stop <area-uuid> --json
huego geofence-client at-home <client-uuid> false --json
huego behavior-script list --json
huego behavior-instance list --json
huego behavior-instance disable <instance-uuid> --json
huego behavior-instance update <instance-uuid> --file automation-update.json --json
```

Sensor groups expose enable/disable where the pinned schema has `enabled`.
Rename sends a metadata-only update, or the native `name` field for geofence
clients. Motion sensitivity is non-negative; the bridge enforces its reported
`sensitivity_max`. Effects and power-up presets depend on light capabilities;
custom power-up settings, gradients, signaling and multi-field writes use
`light update --file ...`. Power-up configuration does not switch the light on.

Behavior-instance `configuration` is script-specific. Inspect the associated
behavior script's configuration schema before constructing an update.
Automation creation/deletion is absent from this public OpenHue revision,
so obtain the appropriate payload from the current official Hue reference
and opt into an explicit raw operation if the bridge supports it:

```sh
huego resource create behavior_instance --raw --file automation.json --dry-run --json
huego resource create behavior_instance --raw --file automation.json --json
```

## Coverage and firmware extensions

All 41 types in the pinned resource contract have collection/item reads and the
documented POST/PUT/DELETE operations. This includes lighting, rooms, zones,
scenes, smart scenes, sensors, MotionAware resources, devices, connectivity,
entertainment configuration, geofencing, automations, HomeKit and Matter.
Read-only resources do not expose direct update/create/delete commands.

The public specification has gaps, including automation creation, full
entertainment-area creation, some integration writes, MotionAware setup and
new `switch_input_configuration` firmware resources. `--raw` bypasses only
the operation catalog, retaining JSON-object and path validation:

```sh
huego resource list switch_input_configuration --raw --json
huego resource update switch_input_configuration <service-uuid> --raw --file switch.json --json
```

The [official Hue API reference](https://developers.meethue.com/develop/hue-api-v2/api-reference/)
is authoritative for current hardware behavior. The CLI validates operation
availability in the catalog, payload syntax, and convenience-command values;
it does not perform full schema or capability validation. The bridge remains
the authority for semantic validation. The embedded schemas are intentionally
unmodified interface facts, including upstream omissions and inconsistencies.

This change covers REST configuration, not DTLS entertainment frame streaming
or SSE event subscriptions. Automated verification uses fake HTTPS bridges and
does not modify real devices. Hardware-dependent operations still need testing
on a compatible bridge.
