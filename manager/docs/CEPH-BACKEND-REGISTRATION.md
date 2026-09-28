# Ceph backend registration

Manager registers two **native-managed** storage records only after the Ceph
workflow proves access from every hypervisor: **VM Alpha Storage** (RBD) and
**VM Alpha NFS** (a CephFS-backed NFS export). These are inventory records, not
Kubernetes StorageClasses; no CSI driver or Kubernetes secret is implied.

The Ceph workflow produces a sanitized registration manifest outside the source
tree, such as `/var/lib/vmalpha/storage-registration.json`. It contains no Ceph
keys, passwords, or client material. Transfer it through an authenticated
administrative channel, then run on Manager:

```bash
register-ceph-backends.py --manifest /secure/storage-registration.json
systemctl restart vmalpha-manager
```

The manifest must be a root-owned regular file with exact mode `0600` (not a
symlink), and its `verified_at` timestamp must be within 15 minutes. This
prevents an old success result from remaining online indefinitely. Manager
reports a native backend as `unknown` after that window until a fresh verified
manifest is applied. Generic Manager mount, unmount, and delete actions reject
these records; the Ceph workflow remains their controller.

```json
{
  "version": 1,
  "status": "verified",
  "verified_at": "2026-09-27T22:30:00Z",
  "fsid": "...",
  "monitors": ["192.168.71.81:6789"],
  "placement_nodes": ["kvm11.vmalpha.com", "kvm12.vmalpha.com", "kvm13.vmalpha.com"],
  "checks": {"rbd_read_write": true, "cephfs_read_write": true, "nfs_read_write": true},
  "host_access": {
    "kvm11.vmalpha.com": {"rbd_read_write": true, "nfs_read_write": true},
    "kvm12.vmalpha.com": {"rbd_read_write": true, "nfs_read_write": true},
    "kvm13.vmalpha.com": {"rbd_read_write": true, "nfs_read_write": true}
  },
  "rbd_pool": "vmalpha-rbd",
  "cephfs": {"name":"vmalpha-fs","data_pool":"vmalpha-cephfs-data","metadata_pool":"vmalpha-cephfs-metadata"},
  "nfs": {"service":"vmalpha-nfs","export":"/vmalpha","endpoint":"approved-vip:2049"}
}
```

`nfs.endpoint` is one approved stable endpoint. Registration deliberately
refuses a multi-host direct endpoint list, an unapproved placeholder, or an
unverified NFS service. Native RBD credentials stay inside the protected host
workflow and are never copied into Manager state.

## Refresh automation

Manager refreshes through the existing pinned native-host broker every five
minutes. It requests only the allowlisted `ceph.telemetry` / `storage
registration` result from kvm11; the Manager service writes its own protected
state under `/opt/vmalpha-manager/data` and never needs write access to a
root-owned host path. The Ceph verifier must preserve its real `verified_at`
time and refresh its evidence after a successful initial RBD/NFS read/write
verification. A broker failure, stale evidence, or invalid manifest does not
overwrite the prior state. Its original verification timestamp makes Manager
show the backends as `unknown` after 15 minutes instead of retaining an old
`online` result.
