# Operator checks — development installation

These checks were run on the isolated single-node integration fixture. Commands
that inspect Kubernetes use a root-protected administrator kubeconfig; do not
copy it to dashboards, user home folders, chat or a reusable appliance. Installed
cluster tools live in `/usr/local/bin`; include that directory in the operator's
PATH when using a restricted sudo environment.

## Read-only health checks

```sh
hostnamectl
getenforce
systemctl is-active firewalld cockpit.socket
/usr/local/bin/kubectl --kubeconfig /etc/kubernetes/admin.conf get nodes -o wide
/usr/local/bin/kubectl --kubeconfig /etc/kubernetes/admin.conf get pods -A
/usr/local/bin/kubectl --kubeconfig /etc/kubernetes/admin.conf get pvc,pv
ceph -s
virsh -c qemu:///system list --all
virsh -c storage:///system pool-list --all
systemctl is-active vmalpha-prometheus vmalpha-node-exporter vmalpha-libvirt-exporter vmalpha-ceph-exporter
```

Healthy service processes alone do not establish healthy data. Check collector
availability and recent measured samples in Monitor, Ceph quorum/OSD state,
PVC attachment and application-level I/O. Some services exist only after their
feature is configured; an unconfigured feature is not a successfully tested one.

## Monitoring and alerts

The host stores bounded Prometheus history (7 days, capped at 2 GB) and protects
its loopback query/collector endpoints with authentication. The web broker
permits fixed metric queries and checks host or VM permissions. It does not
accept arbitrary PromQL. The GUI exposes host-level active pending/firing
alerts only to administrators; a VM-scoped user cannot retrieve other host data.

A real test stopping only the libvirt exporter produced CollectorUnavailable,
then resolved automatically after that exporter restarted. Stopping the exporter
does not stop its VMs. An unavailable alert evaluator must be displayed as
unavailable, not as proof that no alerts exist. Current supplied rules cover
collector availability, host memory pressure and low filesystem space; there
is no external notification integration or complete custom-rule editor yet.

The chart timestamp is the last plotted evaluation point. It is not a guarantee
that an exporter sampled that exact instant. Collector health must be evaluated
alongside retained history. No-data, missing hardware counters and unavailable
physical FC instrumentation must not be converted into invented zero values.

## Storage safety

Use the Host Client's discovered devices, not guessed Linux device letters.
The acceptance disks were new disposable disks/volumes. Do not repeat their
formatting scripts on arbitrary hosts. Before disconnecting a datastore, verify
that no defined or running guest still references it. Retain PVCs and virtual
disks when stopping workloads unless removal was explicitly selected.

RWO is not permission to mount one filesystem from two active writers. Allow
CSI to detach and reattach during movement and inspect the previous writer
before considering force actions. NFS and iSCSI tests used dedicated loopback
servers; they do not verify external fabrics, redundant paths or SAN failover.
The native RBD driver uses a restricted cephx identity and the signed narrow
compatibility package; authentication is not disabled.

## Backup, recovery and rollback

Keep versioned media/checksums, configuration backups and application data
backups separately from the running host. Cluster backups contain credentials
and encryption material; encrypt and restrict them, and test restoration on
isolated targets before relying on them. An etcdutl snapshot-status check passed
for the pre-expansion snapshot, but an isolated restore drill is still pending.

The tested second host reboot preserved the CSI marker, native RBD/NFS/iSCSI
guest data and monitoring history after orderly shutdown configuration was
corrected. Wait for Ceph and CSI leadership/attachment recovery and inspect
protected logs. Do not remove finalizers or reset disks to hide a slow recovery.

Rollback by overwriting an initialized host with an old installer is destructive
and is not the supported application rollback procedure. Preserve existing VMs
and disks, restore compatible configuration only through a tested recovery plan,
and verify data and access after any rollback. Complete product upgrade,
rollback and cluster-member restore procedures remain release gates.
