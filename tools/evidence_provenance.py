#!/usr/bin/env python3
"""Build and verify immutable evidence-run manifests without interpreting payloads.

The tool records file-level provenance only. It does not derive browser
configuration, parse fingerprint values, access a network, or mutate source data.
"""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tempfile
import uuid

SCHEMA_VERSION = 1
CHUNK_SIZE = 1024 * 1024
RUN_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")
STAGE_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")
STATUS_VALUES = {"success", "partial", "failed", "skipped", "unknown"}


def canonical_bytes(value):
    return json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")


def digest_json(value):
    return hashlib.sha256(canonical_bytes(value)).hexdigest()


def now_utc():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def is_link_or_reparse(st):
    return stat.S_ISLNK(st.st_mode) or bool(
        getattr(st, "st_file_attributes", 0)
        & getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0x400)
    )


def file_signature(st):
    return (st.st_dev, st.st_ino, st.st_size, st.st_mtime_ns)


def normalize_relative_path(raw):
    if not isinstance(raw, str):
        raise ValueError("artifact path must be a string")
    text = raw.replace("\\", "/").strip()
    path = PurePosixPath(text)
    if (
        not text
        or path.is_absolute()
        or any(part in ("", ".", "..") for part in path.parts)
        or ":" in path.parts[0]
        or "\x00" in text
    ):
        raise ValueError("artifact path must be a safe path relative to root: " + repr(raw))
    return path.as_posix()


def resolve_artifact(root, relative):
    relative = normalize_relative_path(relative)
    candidate = root.joinpath(*PurePosixPath(relative).parts)
    current = root
    for part in PurePosixPath(relative).parts:
        current = current / part
        st = current.lstat()
        if is_link_or_reparse(st):
            raise ValueError("artifact path crosses a link or reparse point: " + relative)
    resolved = candidate.resolve(strict=True)
    if resolved != root and root not in resolved.parents:
        raise ValueError("artifact escapes input root: " + relative)
    return relative, resolved


def hash_artifact(root, relative):
    relative, path = resolve_artifact(root, relative)
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode):
        raise ValueError("artifact is not a regular file: " + relative)
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        opened = os.fstat(handle.fileno())
        if file_signature(opened) != file_signature(before):
            raise ValueError("artifact changed before read: " + relative)
        for chunk in iter(lambda: handle.read(CHUNK_SIZE), b""):
            digest.update(chunk)
        after_handle = os.fstat(handle.fileno())
    after_path = path.lstat()
    if (
        is_link_or_reparse(after_path)
        or file_signature(after_handle) != file_signature(before)
        or file_signature(after_path) != file_signature(before)
    ):
        raise ValueError("artifact changed during read: " + relative)
    return {
        "path": relative,
        "size": before.st_size,
        "mtime_ns": before.st_mtime_ns,
        "sha256": digest.hexdigest(),
    }


def read_json(path):
    with path.open(encoding="utf-8") as handle:
        return json.load(handle)


def validate_id(value, pattern, label):
    if not isinstance(value, str) or not pattern.fullmatch(value):
        raise ValueError("invalid " + label)
    return value


def parse_time(value, label):
    if value in (None, ""):
        return None
    if not isinstance(value, str):
        raise ValueError(label + " must be an ISO-8601 string")
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError("invalid " + label) from exc
    if parsed.tzinfo is None:
        raise ValueError(label + " must include a timezone")
    return parsed


def normalize_metadata(value, label):
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise ValueError(label + " must be an object")
    out = {}
    for key, item in value.items():
        if not isinstance(key, str) or not key or len(key) > 64:
            raise ValueError(label + " contains an invalid key")
        if isinstance(item, (str, int, float, bool)) or item is None:
            out[key] = item
        else:
            raise ValueError(label + " values must be scalar")
    return out


def manifest_digest_payload(manifest):
    return {
        key: value
        for key, value in manifest.items()
        if key != "content_sha256"
    }


def verify_manifest_digest(manifest):
    expected = digest_json(manifest_digest_payload(manifest))
    if expected != manifest.get("content_sha256"):
        raise ValueError("manifest integrity check failed")
    return expected


def build_manifest(root, spec):
    if not isinstance(spec, dict):
        raise ValueError("spec must be a JSON object")
    run_id = spec.get("run_id") or ("run-" + uuid.uuid4().hex)
    validate_id(run_id, RUN_ID_RE, "run_id")
    sample_id = spec.get("sample_id", "")
    if sample_id:
        validate_id(sample_id, RUN_ID_RE, "sample_id")

    raw_stages = spec.get("stages")
    if not isinstance(raw_stages, list) or not raw_stages:
        raise ValueError("spec.stages must be a non-empty array")

    seen_stages = set()
    seen_paths = {}
    stages = []
    last_ordinal = -1
    for index, raw in enumerate(raw_stages):
        if not isinstance(raw, dict):
            raise ValueError("each stage must be an object")
        stage_id = validate_id(raw.get("stage_id"), STAGE_ID_RE, "stage_id")
        if stage_id in seen_stages:
            raise ValueError("duplicate stage_id: " + stage_id)
        seen_stages.add(stage_id)
        ordinal = raw.get("ordinal", index)
        if not isinstance(ordinal, int) or ordinal < 0:
            raise ValueError("stage ordinal must be a non-negative integer")
        if ordinal <= last_ordinal:
            raise ValueError("stage ordinals must be strictly increasing")
        last_ordinal = ordinal
        status = raw.get("execution_status", "unknown")
        if status not in STATUS_VALUES:
            raise ValueError("invalid execution_status for " + stage_id)

        started_at = parse_time(raw.get("started_at"), stage_id + ".started_at")
        completed_at = parse_time(raw.get("completed_at"), stage_id + ".completed_at")
        if started_at and completed_at and completed_at < started_at:
            raise ValueError("completed_at is before started_at for " + stage_id)

        raw_artifacts = raw.get("artifacts", [])
        if not isinstance(raw_artifacts, list):
            raise ValueError("artifacts must be an array for " + stage_id)
        artifacts = []
        for item in raw_artifacts:
            relative = normalize_relative_path(item)
            if relative in seen_paths:
                raise ValueError(
                    "artifact is assigned to multiple stages: "
                    + relative + " (" + seen_paths[relative] + ", " + stage_id + ")"
                )
            seen_paths[relative] = stage_id
            artifacts.append(hash_artifact(root, relative))
        stages.append({
            "stage_id": stage_id,
            "ordinal": ordinal,
            "execution_status": status,
            "started_at": raw.get("started_at", ""),
            "completed_at": raw.get("completed_at", ""),
            "session_ref": str(raw.get("session_ref", "")).strip(),
            "metadata": normalize_metadata(raw.get("metadata"), stage_id + ".metadata"),
            "artifacts": artifacts,
            "artifacts_sha256": digest_json(artifacts),
        })

    reports = {}
    raw_reports = spec.get("reports", {})
    if raw_reports is not None:
        if not isinstance(raw_reports, dict):
            raise ValueError("reports must be an object")
        for name, path in sorted(raw_reports.items()):
            if not isinstance(name, str) or not re.fullmatch(r"[A-Za-z0-9._-]{1,64}", name):
                raise ValueError("invalid report name")
            relative = normalize_relative_path(path)
            reports[name] = hash_artifact(root, relative)

    payload = {
        "schema_version": SCHEMA_VERSION,
        "run_id": run_id,
        "sample_id": sample_id,
        "created_at": now_utc(),
        "scope": "file-level run provenance; payload semantics are not interpreted",
        "metadata": normalize_metadata(spec.get("metadata"), "metadata"),
        "stages": stages,
        "reports": reports,
    }
    payload["content_sha256"] = digest_json(manifest_digest_payload(payload))
    return payload


def verify_manifest(root, manifest):
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise ValueError("unsupported manifest schema")
    validate_id(manifest.get("run_id"), RUN_ID_RE, "run_id")
    sample_id = manifest.get("sample_id", "")
    if sample_id:
        validate_id(sample_id, RUN_ID_RE, "sample_id")
    verify_manifest_digest(manifest)

    checked = 0
    for stage in manifest.get("stages", []):
        stage_id = validate_id(stage.get("stage_id"), STAGE_ID_RE, "stage_id")
        for stored in stage.get("artifacts", []):
            current = hash_artifact(root, stored["path"])
            if current["sha256"] != stored.get("sha256") or current["size"] != stored.get("size"):
                raise ValueError("artifact changed since manifest: " + stored["path"])
            checked += 1
        if digest_json(stage.get("artifacts", [])) != stage.get("artifacts_sha256"):
            raise ValueError("stage artifact list integrity failed: " + stage_id)
    for stored in manifest.get("reports", {}).values():
        current = hash_artifact(root, stored["path"])
        if current["sha256"] != stored.get("sha256") or current["size"] != stored.get("size"):
            raise ValueError("report changed since manifest: " + stored["path"])
        checked += 1
    return checked


def atomic_create(path, value):
    destination = path.resolve()
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists():
        raise FileExistsError(destination)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w", encoding="utf-8", newline="\n",
            dir=destination.parent, prefix=".provenance-", delete=False,
        ) as handle:
            temporary = Path(handle.name)
            json.dump(value, handle, ensure_ascii=False, indent=2, sort_keys=True)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.link(temporary, destination)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def load_manifest(path):
    value = read_json(path)
    if not isinstance(value, dict):
        raise ValueError("manifest must be an object")
    return value


def index_manifests(paths):
    rows = []
    seen = {}
    conflicts = []
    for path in paths:
        manifest = load_manifest(path)
        if manifest.get("schema_version") != SCHEMA_VERSION:
            raise ValueError("unsupported manifest schema: " + str(path))
        run_id = validate_id(manifest.get("run_id"), RUN_ID_RE, "run_id")
        digest = verify_manifest_digest(manifest)
        if run_id in seen and seen[run_id]["content_sha256"] != digest:
            conflicts.append({
                "run_id": run_id,
                "first": seen[run_id]["manifest"],
                "second": str(path),
            })
        row = {
            "run_id": run_id,
            "sample_id": manifest.get("sample_id", ""),
            "manifest": str(path),
            "content_sha256": digest,
            "stage_ids": [s.get("stage_id", "") for s in manifest.get("stages", [])],
        }
        seen[run_id] = row
        rows.append(row)
    rows.sort(key=lambda row: (row["run_id"], row["manifest"]))
    return {
        "schema_version": SCHEMA_VERSION,
        "generated_at": now_utc(),
        "count": len(rows),
        "conflicts": conflicts,
        "runs": rows,
        "runs_sha256": digest_json(rows),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)

    build = commands.add_parser("build", help="build a new immutable run manifest")
    build.add_argument("--root", required=True, type=Path)
    build.add_argument("--spec", required=True, type=Path)
    build.add_argument("--output", required=True, type=Path)

    verify = commands.add_parser("verify", help="verify a manifest and current files")
    verify.add_argument("--root", required=True, type=Path)
    verify.add_argument("--manifest", required=True, type=Path)

    index = commands.add_parser("index", help="index existing run manifests")
    index.add_argument("--manifests", required=True, nargs="+", type=Path)
    index.add_argument("--output", required=True, type=Path)

    args = parser.parse_args()
    try:
        if args.command in ("build", "verify"):
            root_input_st = args.root.lstat()
            if is_link_or_reparse(root_input_st) or not stat.S_ISDIR(root_input_st.st_mode):
                raise ValueError("root must be a real directory, not a link/reparse point")
            root = args.root.resolve(strict=True)
        if args.command == "build":
            output_path = args.output.resolve()
            if output_path == root or root in output_path.parents:
                raise ValueError("output must be outside the input root")
            spec = read_json(args.spec)
            manifest = build_manifest(root, spec)
            atomic_create(args.output, manifest)
            print(json.dumps({
                "run_id": manifest["run_id"],
                "stages": len(manifest["stages"]),
                "content_sha256": manifest["content_sha256"],
            }))
        elif args.command == "verify":
            manifest = load_manifest(args.manifest)
            checked = verify_manifest(root, manifest)
            print(json.dumps({
                "run_id": manifest["run_id"],
                "verified_files": checked,
                "content_sha256": manifest["content_sha256"],
            }))
        else:
            manifest_paths = [path.resolve(strict=True) for path in args.manifests]
            output = index_manifests(manifest_paths)
            atomic_create(args.output, output)
            print(json.dumps({
                "count": output["count"],
                "conflicts": len(output["conflicts"]),
                "runs_sha256": output["runs_sha256"],
            }))
            if output["conflicts"]:
                return 2
        return 0
    except (OSError, ValueError, KeyError, TypeError, json.JSONDecodeError) as exc:
        parser.exit(1, "provenance failed: " + str(exc) + "\n")


if __name__ == "__main__":
    raise SystemExit(main())
