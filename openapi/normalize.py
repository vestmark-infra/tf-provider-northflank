#!/usr/bin/env python3
"""
normalize.py - Convert the Northflank hybrid Swagger-2/OpenAPI-3 spec into a
clean OpenAPI 3.0.3 document that oapi-codegen can consume.

Problems in the raw spec:
  1. Operations are defined in components.pathItems and $ref'd from paths —
     OpenAPI 3.1 pattern used in a declared-3.0.1 document. We inline them.
  2. Responses use Swagger-2-style `schema` instead of OA3 `content`.
  3. Path/query parameters duplicate Swagger-2 fields (type, in, name, required,
     description, example) *inside* their nested `schema` object.
  4. No securitySchemes are declared (Bearer auth must be added).

Usage:
    python3 normalize.py <input.json> <output.json>
"""

import copy
import json
import sys


SWAGGER2_PARAM_CRUFT = {"in", "name", "required", "type", "description", "example"}


def normalize_param(param: dict) -> dict:
    """Strip Swagger-2 cruft from the nested schema object inside a parameter."""
    p = copy.deepcopy(param)
    schema = p.get("schema")
    if isinstance(schema, dict):
        for key in SWAGGER2_PARAM_CRUFT:
            schema.pop(key, None)
        # If schema ended up empty or only has type from the outer param, rebuild it
        if not schema:
            p["schema"] = {"type": p.get("type", "string")}
    else:
        # No schema at all — build one from swagger2 fields
        s: dict = {}
        if "type" in p:
            s["type"] = p["type"]
        if "enum" in p:
            s["enum"] = p["enum"]
        if "minimum" in p:
            s["minimum"] = p["minimum"]
        if "maximum" in p:
            s["maximum"] = p["maximum"]
        if "pattern" in p:
            s["pattern"] = p["pattern"]
        if "minLength" in p:
            s["minLength"] = p["minLength"]
        if "maxLength" in p:
            s["maxLength"] = p["maxLength"]
        p["schema"] = s if s else {"type": "string"}
    # Remove top-level Swagger-2 duplicates that belong on schema only
    for key in ("type", "enum", "minimum", "maximum", "pattern", "minLength", "maxLength"):
        p.pop(key, None)
    # OA3 `examples` must be a map<string, Example>, not an array.
    # Swagger-2 uses an array; convert it to a map keyed by index string.
    examples = p.get("examples")
    if isinstance(examples, list):
        p["examples"] = {str(i): {"value": v} for i, v in enumerate(examples)}
    return p


def normalize_response(resp: dict) -> dict:
    """Convert Swagger-2 `schema` in a response to OA3 `content`."""
    r = copy.deepcopy(resp)
    if "schema" in r:
        schema = r.pop("schema")
        r["content"] = {"application/json": {"schema": schema}}
    if "description" not in r:
        r["description"] = "Response"
    return r


def normalize_operation(op: dict) -> dict:
    """Normalize a single operation object."""
    o = copy.deepcopy(op)
    # Strip Northflank-specific extension fields that confuse generators
    o.pop("x-nf-team-scoped", None)
    o.pop("x-nf-cli-command", None)
    # Normalize parameters
    o["parameters"] = [normalize_param(p) for p in o.get("parameters", [])]
    # Normalize responses
    new_responses: dict = {}
    for code, resp in o.get("responses", {}).items():
        new_responses[code] = normalize_response(resp)
    o["responses"] = new_responses
    return o


def main(input_path: str, output_path: str) -> None:
    with open(input_path) as f:
        raw = json.load(f)

    path_items: dict = raw.get("components", {}).get("pathItems", {})
    raw_paths: dict = raw.get("paths", {})

    # Build normalized paths.
    # The raw spec structure: each HTTP method value inside a path is a {"$ref": "..."} that
    # points to a component in components.pathItems.  Each such component is an operation object
    # (has operationId, parameters, requestBody, responses) — NOT a nested path item in the
    # traditional sense.  We resolve each method-level $ref to its component and normalize it.
    #
    # We drop most /v1/teams/{teamId}/... paths: they are exact mirrors of the non-team paths
    # (with an extra `teamId` path parameter) and share operationIds, causing duplicate type
    # declarations in the generated code.
    #
    # We do keep two team-specific paths that enable multi-team workflows:
    #   /v1/teams                          — list teams visible to the token
    #   /v1/teams/{teamId}/projects        — list projects scoped to a specific team
    # The team-scoped project list is assigned a unique operationId (getTeamProjects) to avoid
    # collision with the token-implicit getProjects.
    TEAM_SCOPED_ALLOWLIST = {"/v1/teams", "/v1/teams/{teamId}/projects"}
    TEAM_SCOPED_OPERATION_ID_OVERRIDES = {
        # path → method → new operationId
        "/v1/teams/{teamId}/projects": {"get": "getTeamProjects"},
    }

    http_methods = {"get", "post", "put", "patch", "delete", "head", "options", "trace"}

    new_paths: dict = {}
    for path, path_obj in raw_paths.items():
        if path.startswith("/v1/teams/") and path not in TEAM_SCOPED_ALLOWLIST:
            continue
        resolved: dict = {}
        for key, val in path_obj.items():
            if key not in http_methods:
                # path-level fields (parameters, summary, etc.) — copy as-is
                resolved[key] = copy.deepcopy(val)
                continue
            # val is either {"$ref": "..."} or an inline operation object
            if isinstance(val, dict) and "$ref" in val:
                ref_key = val["$ref"].split("/")[-1]
                op = path_items.get(ref_key, {})
            else:
                op = val
            op_normalized = normalize_operation(op)
            # Apply any operationId override for this path+method.
            override_id = TEAM_SCOPED_OPERATION_ID_OVERRIDES.get(path, {}).get(key)
            if override_id:
                op_normalized["operationId"] = override_id
            resolved[key] = op_normalized

        if any(k in resolved for k in http_methods):
            new_paths[path] = resolved

    out = {
        "openapi": "3.0.3",
        "info": {
            "title": "Northflank API",
            "version": raw.get("info", {}).get("version", "1.0.0"),
        },
        "servers": [{"url": "https://api.northflank.com"}],
        "security": [{"bearerAuth": []}],
        "paths": new_paths,
        "components": {
            "schemas": raw.get("components", {}).get("schemas", {}),
            "securitySchemes": {
                "bearerAuth": {
                    "type": "http",
                    "scheme": "bearer",
                    "description": (
                        "Northflank API token. Create one in Team Settings → API → Tokens."
                    ),
                }
            },
        },
    }

    with open(output_path, "w") as f:
        json.dump(out, f, indent=2)

    # Stats
    total_paths = len(out["paths"])
    total_ops = sum(
        1
        for pi in out["paths"].values()
        for m in http_methods
        if m in pi
    )
    print(f"  paths: {total_paths}, operations: {total_ops}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <input.json> <output.json>", file=sys.stderr)
        sys.exit(1)
    main(sys.argv[1], sys.argv[2])
