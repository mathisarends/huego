"""Extract the CLIP resource contract from a pinned local OpenHue checkout.

Run with Python and PyYAML: python scripts/update-api-catalog.py /path/to/openhue-api
"""

import json
from pathlib import Path
import subprocess
import sys

import yaml


def resolve(value, base, stack=()):
    if isinstance(value, list):
        return [resolve(item, base, stack) for item in value]
    if not isinstance(value, dict):
        return value
    if "$ref" in value:
        filename, _, fragment = value["$ref"].partition("#")
        target = (base.parent / filename).resolve() if filename else base
        reference = (target, fragment)
        if reference in stack:
            raise ValueError(f"Recursive schema: {reference}")
        document = yaml.safe_load(target.read_text(encoding="utf-8"))
        for segment in fragment.strip("/").split("/") if fragment else []:
            document = document[segment.replace("~1", "/").replace("~0", "~")]
        result = resolve(document, target, (*stack, reference))
        result.update(resolve({k: v for k, v in value.items() if k != "$ref"}, base, stack))
        return result
    return {key: resolve(item, base, stack) for key, item in value.items()}


def schema_facts(schema):
    if isinstance(schema, list):
        return [schema_facts(item) for item in schema]
    if not isinstance(schema, dict):
        return schema
    return {
        key: ({name: schema_facts(item) for name, item in value.items()}
              if key == "properties" else schema_facts(value))
        for key, value in schema.items()
        if key not in {"description", "title", "example", "examples", "externalDocs"}
    }


def main():
    checkout = Path(sys.argv[1]).resolve()
    entry = checkout / "src/main.yaml"
    source = yaml.safe_load(entry.read_text(encoding="utf-8"))
    resources = {}
    for path, reference in source["paths"].items():
        prefix = "/clip/v2/resource/"
        if not path.startswith(prefix):
            continue
        kind, _, item_id = path[len(prefix):].partition("/")
        capability = resources.setdefault(kind, {
            "resource_type": kind, "collection_methods": [], "item_methods": [],
            "request_schemas": {},
        })
        endpoint = resolve(reference, entry)
        methods = capability["item_methods" if item_id else "collection_methods"]
        for method in ("get", "post", "put", "delete"):
            if method not in endpoint:
                continue
            methods.append(method.upper())
            body = endpoint[method].get("requestBody")
            if body:
                capability["request_schemas"][method.upper()] = schema_facts(
                    body["content"]["application/json"]["schema"]
                )
    catalog = {
        "source": "https://github.com/openhue/openhue-api",
        "revision": subprocess.check_output(
            ["git", "-C", str(checkout), "rev-parse", "HEAD"], text=True
        ).strip(),
        "resources": [resources[kind] for kind in sorted(resources)],
    }
    destination = Path(__file__).resolve().parents[1] / "internal/hue/api_catalog.json"
    destination.write_text(json.dumps(catalog, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
