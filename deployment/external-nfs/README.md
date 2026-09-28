# External NFS and Manager datastore browser

This opt-in workflow installs an NFS client and a persistent systemd automount; it does not format disks, change host networking, deploy Kubernetes, or alter Ceph pools/exports. Server/export/id are configurable. `example.yml` describes the requested external server, not a compiled default.

```sh
ansible-playbook -i inventory.ini external-nfs.yml -e @example.yml
```

Inventory group `external_nfs_hosts` contains only intended clients. Run once against the selected hypervisors and, to enable browsing, against Manager too. For the Manager-only run set `external_nfs.manager_browser: true`; this adds a narrowly scoped systemd ReadWritePaths override and restarts Manager when the override changes. Export permissions must allow its `vmalpha` service UID/GID. Mount root is fixed at `/var/lib/vmalpha/datastores/<id>`. NFSv3 uses `vers=3,soft,nofail,_netdev,x-systemd.automount,x-systemd.mount-timeout=30s`. The installer preserves fstab with a backup, refuses conflicting mounts/nonempty local directories, starts automount and verifies the actual mount. Soft mounts can return I/O errors during server outages; this is the explicitly requested policy. No guest disk placement is changed.

After deploying the broker source and verifying selected hypervisor mounts, authenticated storage-create users register via:

```json
POST /api/v1/storage/external-nfs
{"id":"external-nfs","name":"External NFS Datastore","server":"192.168.71.50","export_path":"/nfs","hosts":["kvm11","kvm12","kvm13"]}
```

The endpoint performs read-only pinned broker `storage.external-nfs.status` checks. It does not mount anything. Existing Ceph records are preserved. Reposting the same id/server/export revalidates and refreshes its evidence; changing source requires a separate datastore. Evidence expires after 15 minutes and then reports Unknown until refreshed. Deploy automation should revalidate periodically if continuous online status is required. No Kubernetes storage class is created.

## Browser

Storage → Backends → Browse supports this external datastore and verified Ceph-backed **VM Alpha NFS**. RBD remains unsupported because it has no file hierarchy. The Manager service must see the mount in its own mount namespace and have export permissions. Do not mount an unrelated path or broaden exports to all clients. For Ceph NFS, authorize the actual Manager client explicitly and apply `ceph-browser-example.yml` to Manager only, using the registered stable endpoint. Ceph's browser mount is `/var/lib/vmalpha/datastores/ceph-<service>` and NFSv4.1; external NFS uses NFSv3. Both require soft, nofail and automount setup. The source neither changes the live Ceph export nor starts these mounts automatically.

API under `/api/v1/storage/backends/:id/browser`:

- GET `?path=relative/folder`: list up to 1000 entries.
- GET `/download?path=relative/file`: download a regular file.
- POST `/folder?path=relative/new-folder`: create one directory.
- POST `/upload?path=relative/file`: raw binary body, 512 MiB limit, exclusive creation (no overwrite).
- DELETE `/entry?path=relative/file`: remove a file or an empty folder; never recursive.

Every operation validates registered source, protocol, soft mount, active automount and the opened NFS descriptor. Go `os.Root` confines operations even during symlink races; parent traversal, absolute paths, symlink selections, devices and root deletion are rejected. Existing storage RBAC list/create/delete permissions apply. File deletion is an explicit irreversible action: users must not delete files used by VMs. No background file mutation occurs.

Build integration: include `deployment/external-nfs` and the updated `hypervisor/vmalpha-api.py` in source payloads. The opt-in playbook must be invoked after deployment, not during ISO image creation. Manager requires the normal source rebuild to expose API/UI. This change has source tests; live mounts, export ACLs, browser operations and registration are separate deployment validation.
