#!/usr/bin/env python3
"""Register only fresh, agent-verified native Ceph backends in Manager.

The input is a sanitized manifest written by the Ceph workflow.  It contains no
Ceph keys or passwords.  This script deliberately refuses a manifest until its
per-host RBD and NFS read/write checks have completed.
"""
import argparse
import json
import os
import stat
import tempfile
from datetime import datetime, timezone

REQUIRED_CHECKS = ("rbd_read_write", "cephfs_read_write", "nfs_read_write")
REQUIRED_HOSTS = ("kvm11.vmalpha.com", "kvm12.vmalpha.com", "kvm13.vmalpha.com")


def die(message):
    raise SystemExit(f"registration refused: {message}")


def text(value, field):
    if not isinstance(value, str) or not value.strip():
        die(f"missing {field}")
    return value.strip()


def load_manifest(path, max_age_seconds):
    try:
        info = os.lstat(path)
    except OSError as exc:
        die(f"cannot stat manifest: {exc}")
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode):
        die("manifest must be a regular file, not a symlink")
    if info.st_uid != 0:
        die("manifest must be owned by root")
    if stat.S_IMODE(info.st_mode) != 0o600:
        die("manifest permissions must be exactly 0600")
    try:
        with open(path, encoding="utf-8") as manifest:
            data = json.load(manifest)
    except (OSError, json.JSONDecodeError) as exc:
        die(f"cannot read manifest: {exc}")
    if data.get("version") != 1 or data.get("status") != "verified":
        die("manifest must be version 1 with status=verified")
    verified_at = text(data.get("verified_at"), "verified_at")
    try:
        when = datetime.fromisoformat(verified_at.replace("Z", "+00:00"))
    except ValueError:
        die("verified_at must be RFC3339")
    if when.tzinfo is None:
        die("verified_at must include a timezone")
    age = (datetime.now(timezone.utc) - when.astimezone(timezone.utc)).total_seconds()
    if age < -60 or age > max_age_seconds:
        die(f"verified_at is not fresh (age {age:.0f}s, limit {max_age_seconds}s)")
    return data, verified_at


def validated_backends(manifest, verified_at):
    checks = manifest.get("checks", {})
    if any(checks.get(check) is not True for check in REQUIRED_CHECKS):
        die("all RBD, CephFS, and NFS read/write checks must be true")
    fsid = text(manifest.get("fsid"), "fsid")
    monitors = manifest.get("monitors")
    if not isinstance(monitors, list) or not monitors or not all(isinstance(item, str) and item.strip() for item in monitors):
        die("monitors must be a non-empty string list")
    placement = manifest.get("placement_nodes")
    if not isinstance(placement, list) or not placement:
        die("placement_nodes must be a non-empty list")
    host_access = manifest.get("host_access")
    if not isinstance(host_access, dict):
        die("host_access must contain per-host access evidence")
    for host in REQUIRED_HOSTS:
        evidence = host_access.get(host)
        if not isinstance(evidence, dict) or evidence.get("rbd_read_write") is not True or evidence.get("nfs_read_write") is not True:
            die(f"host_access.{host} must prove RBD and NFS read/write")

    rbd_pool = text(manifest.get("rbd_pool"), "rbd_pool")
    cephfs = manifest.get("cephfs", {})
    fs_name = text(cephfs.get("name"), "cephfs.name")
    data_pool = text(cephfs.get("data_pool"), "cephfs.data_pool")
    metadata_pool = text(cephfs.get("metadata_pool"), "cephfs.metadata_pool")
    nfs = manifest.get("nfs", {})
    service = text(nfs.get("service"), "nfs.service")
    export = text(nfs.get("export"), "nfs.export")
    endpoint = text(nfs.get("endpoint"), "nfs.endpoint")

    now = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    common = {
        "managed_by": "ceph-registration",
        "native_managed": True,
        "fsid": fsid,
        "monitors": monitors,
        "verified_at": verified_at,
        "placement_nodes": placement,
        "host_access": host_access,
    }
    return [
        {
            "id": f"ceph-rbd-{fsid}-{rbd_pool}", "type": "ceph", "name": "VM Alpha Storage",
            "storage_class": "native-rbd", "status": "online",
            "message": "Agent-verified native RBD read/write", "created_at": now,
            "config": dict(common, pool=rbd_pool, access="native-rbd"),
        },
        {
            "id": f"ceph-nfs-{fsid}-{service}", "type": "nfs", "name": "VM Alpha NFS",
            "storage_class": "native-ceph-nfs", "status": "online",
            "message": "Agent-verified Ceph-backed NFS read/write", "created_at": now,
            "config": dict(common, service=service, export_path=export, endpoint=endpoint,
                           access="native-nfs", cephfs={"name": fs_name, "data_pool": data_pool, "metadata_pool": metadata_pool}),
        },
    ]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--storage-config", default="/opt/vmalpha-manager/data/storage.json")
    parser.add_argument("--max-age-seconds", type=int, default=900)
    args = parser.parse_args()
    if args.max_age_seconds <= 0:
        die("max-age-seconds must be positive")
    manifest, verified_at = load_manifest(args.manifest, args.max_age_seconds)
    backends = validated_backends(manifest, verified_at)

    old = {"backends": []}
    if os.path.exists(args.storage_config):
        with open(args.storage_config, encoding="utf-8") as existing:
            old = json.load(existing)
    new_ids = {backend["id"] for backend in backends}
    # Keep legacy and other-cluster registrations. Replace only this FSID's
    # native registrations, so independent Ceph clusters remain intact.
    preserved = []
    for backend in old.get("backends", []):
        config = backend.get("config", {})
        if config.get("managed_by") == "ceph-registration" and config.get("fsid") == manifest["fsid"]:
            continue
        if backend.get("id") not in new_ids:
            preserved.append(backend)
    output = {"backends": preserved + backends}

    directory = os.path.dirname(args.storage_config)
    os.makedirs(directory, mode=0o750, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=".storage.", dir=directory)
    with os.fdopen(descriptor, "w", encoding="utf-8") as state:
        json.dump(output, state, indent=2)
        state.write("\n")
    os.chmod(temporary, 0o600)
    os.replace(temporary, args.storage_config)
    print(json.dumps({"registered": [backend["id"] for backend in backends], "status": "online", "verified_at": verified_at}))


if __name__ == "__main__":
    main()
