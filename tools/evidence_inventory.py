#!/usr/bin/env python3
"""Local, read-only evidence inventory. Never sends data over a network.

The report contains relative file names, byte sizes, nanosecond timestamps,
and SHA-256 digests. It deliberately does not parse document contents,
interpret browser observations, extract credentials, or generate profiles.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import stat
import tempfile

SCHEMA = 1
CHUNK_SIZE = 1024 * 1024


def reparse_or_link(st):
    return stat.S_ISLNK(st.st_mode) or bool(
        getattr(st, "st_file_attributes", 0)
        & getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0x400)
    )


def signature(st):
    return (st.st_dev, st.st_ino, st.st_size, st.st_mtime_ns)


def fingerprint_file(path, relative):
    row = {"path": relative, "status": "unverified"}
    try:
        before = path.lstat()
        if reparse_or_link(before) or not stat.S_ISREG(before.st_mode):
            row["status"] = "skipped_special"
            return row
        digest = hashlib.sha256()
        with path.open("rb") as handle:
            opened = os.fstat(handle.fileno())
            if signature(opened) != signature(before):
                row["status"] = "changed_during_scan"
                return row
            for chunk in iter(lambda: handle.read(CHUNK_SIZE), b""):
                digest.update(chunk)
            after_handle = os.fstat(handle.fileno())
        after_path = path.lstat()
        if (
            reparse_or_link(after_path)
            or signature(before) != signature(after_handle)
            or signature(before) != signature(after_path)
        ):
            row["status"] = "changed_during_scan"
            return row
        row.update(
            status="observed",
            size=before.st_size,
            mtime_ns=before.st_mtime_ns,
            sha256=digest.hexdigest(),
        )
    except OSError as exc:
        row.update(status="unreadable", error_type=type(exc).__name__)
    return row


def scan(root):
    records = []
    stack = [root]
    while stack:
        directory = stack.pop()
        try:
            children = sorted(directory.iterdir(), key=lambda p: p.name)
        except OSError as exc:
            records.append({
                "path": directory.relative_to(root).as_posix(),
                "status": "unreadable_directory",
                "error_type": type(exc).__name__,
            })
            continue
        for path in children:
            relative = path.relative_to(root).as_posix()
            try:
                info = path.lstat()
            except OSError as exc:
                records.append({
                    "path": relative,
                    "status": "unreadable",
                    "error_type": type(exc).__name__,
                })
                continue
            if reparse_or_link(info):
                records.append({"path": relative, "status": "skipped_special"})
            elif stat.S_ISDIR(info.st_mode):
                stack.append(path)
            else:
                records.append(fingerprint_file(path, relative))
    return sorted(records, key=lambda row: row["path"])


def canonical_digest(value):
    data = json.dumps(
        value, ensure_ascii=False, sort_keys=True, separators=(",", ":")
    ).encode("utf-8")
    return hashlib.sha256(data).hexdigest()


def compare(old, new):
    if old.get("schema_version") != SCHEMA:
        raise ValueError("unsupported previous report schema")
    previous = old.get("records")
    if not isinstance(previous, list) or canonical_digest(previous) != old.get("records_sha256"):
        raise ValueError("previous report integrity check failed")
    left = {row["path"]: row for row in previous}
    right = {row["path"]: row for row in new}
    if len(left) != len(previous):
        raise ValueError("duplicate paths in previous report")
    result = {
        "added": [], "removed": [], "changed": [], "unchanged": [],
        "unverified": [],
    }
    for path in sorted(left.keys() | right.keys()):
        a, b = left.get(path), right.get(path)
        if (a is not None and a["status"] != "observed") or (
            b is not None and b["status"] != "observed"
        ):
            result["unverified"].append(path)
        elif a is None:
            result["added"].append(path)
        elif b is None:
            result["removed"].append(path)
        elif a["sha256"] == b["sha256"] and a["size"] == b["size"]:
            result["unchanged"].append(path)
        else:
            result["changed"].append(path)
    return result


def write_atomic(destination, report):
    destination.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w", encoding="utf-8", newline="\n",
            dir=destination.parent, prefix=".evidence-", delete=False,
        ) as handle:
            temporary = Path(handle.name)
            json.dump(report, handle, ensure_ascii=False, indent=2, sort_keys=True)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        # Do not overwrite a prior evidence snapshot.
        if destination.exists():
            raise FileExistsError(destination)
        # A hard-link publish fails rather than replacing an existing file.
        os.link(temporary, destination)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("root", type=Path, help="input directory (read only)")
    parser.add_argument("--output", required=True, type=Path, help="new JSON file outside input")
    parser.add_argument("--previous", type=Path, help="optional earlier inventory to compare")
    args = parser.parse_args()

    try:
        if reparse_or_link(args.root.lstat()):
            raise ValueError("input root must not be a symbolic link or reparse point")
        root = args.root.resolve(strict=True)
        if not root.is_dir():
            raise ValueError("input root is not a directory")
        output = args.output.resolve()
        if output == root or root in output.parents:
            raise ValueError("output must be outside the input directory")
        if output.exists():
            raise FileExistsError(output)

        records = scan(root)
        counts = {}
        for row in records:
            counts[row["status"]] = counts.get(row["status"], 0) + 1
        report = {
            "schema_version": SCHEMA,
            "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "scope": "file-level provenance; not proof of an experimental run",
            "complete": all(row["status"] == "observed" for row in records),
            "counts": counts,
            "records_sha256": canonical_digest(records),
            "records": records,
        }
        if args.previous:
            with args.previous.open(encoding="utf-8") as handle:
                previous = json.load(handle)
            report["comparison"] = compare(previous, records)
            report["previous_records_sha256"] = previous["records_sha256"]
        write_atomic(output, report)
        print(json.dumps({"counts": counts, "complete": report["complete"]}))
        return 0 if report["complete"] else 2
    except (OSError, ValueError, KeyError, TypeError) as exc:
        parser.exit(1, "inventory failed: " + str(exc) + "\n")


if __name__ == "__main__":
    raise SystemExit(main())
