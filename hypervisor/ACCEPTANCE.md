# VM Alpha Hypervisor 1.0 — development acceptance record

Updated 2026-09-16. This is not a final release acceptance certificate.
The final 1.0 ISO has not been frozen or clean-installed.

| Area | Verified result | Remaining acceptance |
|---|---|---|
| Kubernetes 1.36.4 / containerd 2.3.5 | Two converged control-plane nodes Ready in the same cluster; Flannel and cross-node CoreDNS operate | Same cluster at 3 and 4 nodes; worker distinction; additional networking/recovery checks |
| Ceph 20.2.4 | Four selected OSDs on two guest hosts; HEALTH_OK; every PG has replicas on distinct guest hosts | Three-/four-node quorum and recovery; actual host-loss test |
| CSI 3.17.1 | RWO data survives recreation, stop/start and node-1 to node-2 rescheduling with unchanged PVC/PV identity | Further node expansion and volume expansion |
| Native RBD | Narrow libvirt authentication-option compatibility patch; authenticated pool/image/VM disk and real I/O pass | Signed narrow driver and source provenance packaged; final ISO inclusion pending |
| NFS | Real sparse disk, VM attachment/I/O, consumer disconnect guard pass | Final clean-install and external-server coverage |
| iSCSI | Explicit loopback CHAP target, LUN discovery/attachment/I/O, consumer guard pass | External SAN and multipath coverage |
| Fibre Channel | Discovery and selected-adapter operations implemented | Physical HBA/fabric not present in current fixture; no physical FC success claimed |
| Monitoring | Host, VM, Kubernetes and Ceph samples; measured CPU load response; TLS/auth and limited-token tests pass | Expanded-node collection, complete alerts UI, final visual and release checks |
| Recovery | Data on CSI/RBD/NFS/iSCSI and Prometheus history survived both forced-reset recovery and the subsequent graceful reboot | First graceful shutdown hung; second reboot passed after graceful-kubelet/CSI-priority fix. Final ISO retest remains |
| Security/UI | Earlier VM-scoped RBAC, console revocation and host control checks recorded | Full current-source regression/feature matrix and final media checks |
| Payloads | Feature RPMs, binaries and images downloaded; offline install exercised | Signed manifest verified on Linux; 331 transferred file checksums pass. Media/installed-OS integration and disconnected first-use test pending |
| Branding | Source contains VM Alpha Hypervisor/Linux 1.0 branding | Exact final ISO BIOS/UEFI installer/boot/login/web audit |

Live expansion is authorized sequentially from the same first node through
four nodes. The guide must also explain a five-node topology, but five-node
validation must not be claimed without running it. Multiple guest nodes on
one physical hypervisor do not establish physical-host high availability.

Evidence resides in the development output directory. In particular:
`csi-persistence.log`, `rbd-driver-install.log`, `storage-guest-test.log`,
`storage-io-after-reboot.log`, `iscsi-connect.log`, `iscsi-resume.log`,
`iscsi-attach.log`, `iscsi-io.log`, `integrated-metrics.log`,
`monitoring-load.log`, `monitoring-access.log`, `host-recovery-verify.log`.

The complete release README, installation/expansion/admin guides, screenshots,
known-gaps matrix and clean-media acceptance report remain required.

## Node 2 installation regression (2026-09-16)

The internal node fixture stopped during post-install because install-product.sh
tried to install a module onto itself in /usr/share/vmalpha. The source now
preserves permissions for same-inode files and copies from separate source
directories. Both paths passed a focused execution check. Replaying the remaining
post-install script returned status 0 and booted the installed VM Alpha login.
Because Anaconda had aborted before completing its finalization, several
services failed and a resolver file retained root_t labeling. Full filesystem
relabel/recovery is in progress. This is not a clean-install pass. Node 2 has
not joined the cluster. The final media must complete Anaconda without recovery.

The pre-expansion etcd snapshot is mode 0600 and etcdutl successfully read its
status (revision 8721, 832 keys, approximately 6.2 MB). This validates snapshot
readability, not a restore drill.

## Alerts and corrected development media

The metrics API now returns real pending/firing Prometheus alerts to host
administrators. VM-scoped readers cannot use this path to read host alerts.
Four focused tests cover denial, malformed/oversized/backend-failure responses
and preservation of real alert states. The integration Host Client displays the
actual CollectorUnavailable pending alert with charts; screenshot recorded as
`monitoring-alert-pending.png`. A stopped libvirt metrics collector produced a
real firing alert and was restarted in the test's finally block. Automatic
resolution passed in `alerts-live-test.log`; the GUI also showed no remaining
pending/firing alert after recovery (`monitoring-alert-resolved.png`).

The corrected internal offline development ISO completed its embedded media
check. It includes the same-file installer correction and monitoring config
preservation. It is test media, not a final release, and does not contain source
changes made after that build (including the new alerts UI and explicit signed
RBD package selection). Transfer and clean-install acceptance remain in progress.

Namespace/pod filtering now returns only the selected workload's actual CPU,
memory, receive/transmit and restart series. All five live API checks passed;
eight local monitoring boundary tests passed. Cluster Ready-node count remains
explicitly cluster-wide. The final ISO has not incorporated these later changes.

Reusable node preparation and protected kubeadm-join helpers are drafted.
Thirteen Linux input/security tests passed (CIDR overlap/source bounds, endpoint
and node identity, CA pinning and role restrictions). They have not yet performed
a live join. No kubeadm reset or membership deletion was executed.

## Resumed installation and packaging checks (2026-09-17 UTC)

The second node-2 installation installed the offline packages but aborted when
monitoring attempted to start services inside the installer chroot. The source
now defers service startup until boot. Replaying the 69-line remaining post
script returned zero; filesystem labeling is being completed manually. This
recovered installation cannot count as clean-media acceptance.

The v3 candidate passed the embedded media checksum but failed content
inspection: `/vmalpha/features` was a symlink to the build server, so the feature
payload was absent. This candidate is rejected and must not be deployed.
The build script now resolves the payload directory before mapping it and
extracts the finished ISO's payload to verify its signed manifest and all file
hashes. A separate v4 development build is in progress. At that checkpoint no
final media pass or second cluster member had been verified.

Subsequent live recovery and two-node checks passed. Node 2 booted with a writable
root filesystem, SELinux Enforcing, no failed units, its default VM pool active,
the patched signed RBD driver installed, and all offline images imported. After
NTP synchronization, the preparation helper applied prerequisites and the join
helper joined the existing cluster as a second control plane. Both nodes are
Ready. The serving-CSR helper verified and approved the new node's identity and
address. The consumed bootstrap token was revoked and its join file removed.

`node02-cross-node-persistence.log` records the original StatefulSet moving from
node 1 to node 2 with unchanged PVC/PV identity and data checksum, and working
cluster DNS. `node02-metrics.log` records both kubelet targets up and healthy
host, VM, container and Ceph metric samples. Ceph host enrollment and the new
explicitly selected blank 20 GiB disk passed. `two-node-placement-monitoring.log`
verifies every PG clean with replicas on two distinct guest hosts and exactly
two Ready nodes with both kubelet collectors up. `two-node-native-storage.log`
verifies the original nested VM's RBD/NFS/iSCSI data hashes after expansion.
Three- and four-node checks, actual host-loss recovery and final clean media
remain pending. Node 3 is created but powered off awaiting verified media.

## Clean v4 and three-node gate (2026-09-17)
The internal v4 lab ISO completed unattended EFI installation on a new node3
OS disk without manual recovery. Installed-platform checks pass: read/write root,
SELinux enforcing, no failed units, root-owned product files, /dev/kvm, correct
runtime and patched RBD versions, active/autostart default VM pool/network, and
verified feature-image imports. This lab media includes fixture automation and
is not the final interactive customer ISO.
Node3 joined the same cluster as its third control plane; all three etcd endpoints
committed health proposals. Verified serving CSR approved and temporary enrollment
material revoked/removed. The existing StatefulSet moved node2 to node3 with the
same PVC/PV identities, both original checksums unchanged and working cluster DNS.
A new blank 20GB disk became the fifth OSD. After transient recovery, Ceph is
HEALTH_OK with three monitor quorum members, all five OSDs up/in, and all PGs
active+clean. Pools now use size3/min_size2, with every PG spanning three guest
hosts. Existing nested VM RBD/NFS/iSCSI checksums remain valid. All three kubelet
collectors and Ready-node metrics pass.
The external installation-selected NTP servers rate-limited first-boot requests.
The standard Rocky NTP pool synchronized successfully and was configured for
future boots on node3. This is an environmental setup adjustment, not an installer
recovery. Monitoring discovery was found to lag up to 20 minutes; source and the
integration collector now discover every minute while rotating short-lived tokens
every 20 minutes. That change is not present in v4 and requires final-media testing.
A readable pre-node4 etcd snapshot is saved; no restore-test claim is made.
Node4 installation is starting; four-node acceptance remains pending. Five-node
expansion is documented only. Physical-host HA and an HA API endpoint are not
established by these tests.
A new CirrOS guest imported through node3's installed product API booted with
nested KVM, obtained DHCP/default-route configuration, and accepted serial-console
login and keyboard input. Its SSH key was pinned from the authenticated console.
Outbound HTTPS through NAT returned HTTP200 with certificate verification enabled,
using an explicit public CA bundle because the minimal CirrOS image lacks it.
