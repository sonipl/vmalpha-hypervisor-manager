# Opt-in Ceph shared storage

The Hypervisor Ansible `shared-storage.yml` workflow requires explicit Ceph and shared-storage flags, exactly three host names/client IPv4 addresses, and an allocation-verified NFS VIP/CIDR. No OSD selection or device formatting occurs here. It creates distinct RBD and CephFS data pools, required metadata, three MDS and NFS placements, and HAProxy/keepalived ingress with a single client endpoint. Ceph also manages an auxiliary `.nfs` configuration pool.

The NFS export permits only selected managed clients with root squash; its dedicated directory is mode0770 owned by Ganesha's anonymous uid4294967294. NFS-Ganesha5+ and HAProxy protocol are required to retain client-IP ACLs through ingress. Existing pool, filesystem, and export definitions must be reviewed for conflicts before reuse.

RBD clients use a pool-scoped Ceph identity; protected keyrings never enter registration manifests. The verifier imports/exports/removes its own uniquely named test image and writes/reads/removes a unique NFS file from every managed host. Its timer refreshes evidence every10minutes; failure leaves the previous timestamp unchanged so Manager expires it. It requires verified SSH host keys and existing authorized orchestration access between the first node and peers.

The Manager retrieves root-owned0600 `/var/lib/vmalpha/storage-registration.json` through the enrolled broker query `ceph.telemetry` / `storage registration`, then validates freshness before registration. This source directory mirrors the deployed Hypervisor Ansible files; service installation belongs to the Hypervisor workflow, Manager registration belongs to the Manager workflow.

Do not call `ceph nfs cluster create` again to convert an existing cluster. Apply an ingress service spec and update the existing NFS backend port instead; preserve the shared RADOS export include and exports. See https://docs.ceph.com/en/tentacle/cephadm/services/nfs/ .
