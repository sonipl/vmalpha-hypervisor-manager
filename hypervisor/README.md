# VM Alpha Hypervisor 1.0 — development candidate

VM Alpha Linux provides a KVM/libvirt host with an authenticated web Host Client,
optional Kubernetes (VM Alpha Container Platform) and Ceph (VM Alpha Storage).
It derives from Rocky Linux; upstream package identities, repository metadata,
copyright and license notices remain intact.

**Test candidate:** the exact final interactive ISO passed clean BIOS and UEFI
installation, nested VM NAT/verified HTTPS and graceful reboot persistence on
both hosts. Central Manager and advanced feature gates remain incomplete. See [the acceptance record](ACCEPTANCE.md) for actual results and
[the expansion guide](docs/EXPANSION.md) for topology and operational limits.

## Installation and first access

The release installer uses online Rocky BaseOS/AppStream repositories for the
base OS. Feature RPMs, binaries and container images are being integrated into
the ISO and installed system so feature activation does not depend on first-use
Internet downloads. A fully disconnected base OS installation is not claimed.

Choose the target disk deliberately in the interactive installer; installation
erases the selected target. Automated lab-only media is explicitly restricted
to a new disposable VM's first disk and is not the customer installer. Preserve
existing data and backups before any installation.

The owner-requested initial administrator is configured in kickstart using a
salted password hash. After installation, use the host's management address at
`https://<host-address>:9090` and the supplied initial administrator credentials.
Verify the server certificate against the host/operator trust source. Establish
unique operator credentials and review access before wider use.

Select a unique hostname and a stable management address, confirm DNS/time and
routing, then inspect storage and available resources before creating VMs or
enrolling cluster roles. Runtime installation does not initialize a cluster or
consume OSD disks. Cluster membership and storage provisioning require explicit
role/disk selection; the general product enrollment workflow is still under
implementation and validation.

## Feature and test boundaries

The Host Client source includes native VM creation/import/power/console and
configuration, storage and network management, account/role controls and host
services. Single-node development tests cover actual nested VM operation,
authenticated native RBD, NFS and iSCSI disk I/O, RWO CSI persistence and reboot
recovery. Physical Fibre Channel and external SAN/multipath acceptance remain
unavailable in the current fixture.

Monitoring uses actual Prometheus history for host, VM, container and Ceph
metrics with authenticated collectors and bounded retention/resources. Missing
or unavailable collectors must remain distinguishable from zero utilization.
Two-node collection and the real alert pending/firing/resolution workflow have
passed development checks. The same cluster's RWO workload moved to node 2 with
unchanged volume identity and data checksum; Ceph now places replicas on two
guest hosts. Sequential same-cluster expansion through four nodes, PVC identity/data
persistence, DNS and four-node monitoring passed. Node 2 required installer
recovery; nodes 3 and 4 were clean lab installs. Final visual/security regression
remains a separate release gate. The current nodes share one physical hypervisor and do
not establish physical-host high availability.

The tested component set is Kubernetes 1.36.4, containerd 2.3.5, Ceph 20.2.4 and
Ceph CSI 3.17.1. The native libvirt RBD compatibility package preserves cephx
and carries the narrow authentication-option fix with source provenance and a
package signature. See `features/libvirt-rbd/` and the signed feature manifest.

Do not infer VMware feature equivalence from similar page names. VMFS and VMware
proprietary management features are not implemented by renaming native KVM
operations. Advanced migration, HA and central Manager workflows require their
own implementation and failure/recovery tests. The requested Manager QCOW2 is a
separate deliverable still in development; no Manager appliance is accepted yet.

## Build and verification

Build only under `/repos/vmalpha-rocky9/builds/<version>` on the authorized build
host. `build.sh` verifies the official base ISO checksum and signed feature
payload, builds BIOS/UEFI boot media, checks its embedded media checksum, extracts
and verifies the embedded payload's signed manifest/file hashes, then writes a
SHA256 file. It refuses to overwrite an existing ISO. No production
build-host package installation or service changes are part of this procedure.

A build checksum proves artifact integrity, not functionality. Release requires
BIOS and UEFI clean installs of that exact ISO, first login/branding checks,
feature activation without external feature downloads, real workload/storage
and network tests, security regression, sequential same-cluster expansion and
operator/recovery documentation. Preserve earlier release artifacts for rollback;
never overwrite an initialized host merely to roll back an application update.

## Licenses and provenance

The vendored browser console includes noVNC 1.6.0 with its LICENSE.txt, AUTHORS
and dependency notices. Vendored Metropolis typography retains its OFL notice.
Bundled packages and sources retain their upstream license and provenance
records. Private build signing keys, live host keys, cluster tokens and fixture
state are not inputs to reusable release images.
