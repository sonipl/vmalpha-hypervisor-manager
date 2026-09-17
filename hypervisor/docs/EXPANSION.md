# VM Alpha expansion and recovery — development guide

Status: 2026-09-17 UTC. Sequential expansion through four Ready nodes in the
same cluster passed: three control-plane nodes and one worker. PVC/PV identity
and all staged data markers survived rescheduling; DNS and authenticated
collection from all four kubelets passed. Ceph is healthy with six OSDs across
four guest hosts, three monitors and three host-separated replicas (min_size 2).
Node 2 required installer recovery; nodes 3 and 4 were clean lab installs.
The separate exact final interactive ISO passed BIOS/UEFI clean install and
graceful reboot/nested-guest persistence. Five-node operation is unverified.

## Roles and topology

A control-plane node runs the Kubernetes API, scheduler, controller manager and
stacked etcd. A worker runs workloads. A storage node contributes explicitly
selected Ceph OSD disks. One machine may have all three roles, but each role has
separate membership, maintenance and recovery procedures.

| Size | Proposed role layout | Operational limit |
|---|---|---|
| 1 | One control-plane/worker/storage node | No host failure tolerance |
| 2 | Transitional expansion stage | Two etcd members require both; not HA |
| 3 | Three control-plane/worker/storage nodes | Requires independent hosts, healthy quorum and a resilient API endpoint before an HA claim |
| 4 | Three control-plane nodes plus one worker; storage as explicitly enrolled | Validated guest topology; shared physical failure domain |
| 5 | Three control-plane nodes plus two workers; storage as explicitly enrolled | Documented target topology only; not tested |

All current lab VMs share one physical hypervisor. Four guest nodes on that
machine still share its failure domain. The lab API endpoint currently resolves
to node 1; additional control-plane members alone do not remove that single
point of failure. A production design needs a separately validated stable API
endpoint and independent power/network/storage failure domains.

## Before adding a node

* Record node hostname, management address, role, CPU/RAM and new disk identities.
  Hostnames and machine identities must be unique; never clone an initialized
  node's etcd, Kubernetes PKI, Ceph identity or runtime state.
* Verify forward name resolution, reachable time sources and synchronized clocks.
  All members must resolve the same API endpoint. A local hosts-file entry is a
  lab dependency and is not a redundant DNS service.
* Install the signed bundled feature payload and verify archived image digests.
  Cluster creation/join is separate from runtime installation.
* Preserve a protected etcd snapshot, current cluster configuration and Ceph
  configuration. These contain secrets and must not be embedded in an ISO or
  reusable appliance. Protect backups, test restores in isolation, and record
  the component versions associated with each snapshot.
* Check current Node Ready conditions, Ceph health/OSD membership, PVC state,
  available storage and monitoring before changing membership.
* Record a data checksum on an existing RWO PVC and a native RBD VM disk. Stop
  expansion at the first failed gate; do not create a new cluster to hide a
  failed join.

## Network requirements to validate

The lab uses authenticated API access on TCP 6443, kubelet HTTPS on TCP 10250,
Flannel VXLAN on UDP 8472, and Ceph monitor/OSD traffic on TCP 3300, 6789 and
6800–7568. Restrict these to the relevant node/pod networks. Additional
control-plane members require protected etcd peer/client communication on TCP
2380/2379. Permit only the necessary member addresses. Keep SELinux enforcing
and firewalld active; do not resolve a failed join by disabling either.

Pod and service CIDRs must not overlap management, storage, VPN or external
routes. The lab uses 10.244.0.0/16 and 10.96.0.0/12 respectively. Verify routing,
MTU and cross-node DNS after every join. These values are lab choices, not
universal defaults suitable for every deployment.

## Sequential acceptance gate

The following preparation and enrollment helpers were exercised for node 2.
Run them as root on the new node. Replace the example name, addresses and CIDRs
with the selected deployment values; the helper rejects overlapping networks.
Omitting `--apply` validates the selection without changing the system.

```sh
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
python3 /usr/share/vmalpha/features/prepare-node.py \
  --name node2.example.internal --node-ip 192.168.10.12 \
  --node-cidr 192.168.10.0/24 --pod-cidr 10.244.0.0/16 \
  --service-cidr 10.96.0.0/12 --api-name cluster.example.internal \
  --api-ip 192.168.10.10 --role control-plane --storage --apply
```

Wait for `vmalpha-feature-images.service` to finish successfully and for
`timedatectl show -p NTPSynchronized --value` to return `yes`. The enrollment
configuration must be a root-owned regular file with mode 0600. It must contain
the existing cluster's endpoint, a short-lived bootstrap token, pinned CA hash,
the prepared node name and containerd socket. A control-plane join additionally
requires its selected local API address, certificate-transfer key, and the
existing encryption-provider configuration at `/etc/kubernetes/encryption.yaml`
with mode 0600. Transfer these through authenticated encrypted administration
channels; do not paste them into chat, task logs, an ISO, or a reusable image.

```sh
python3 /usr/share/vmalpha/features/join-node.py --config-file /root/join.json
python3 /usr/share/vmalpha/features/join-node.py --config-file /root/join.json --apply
```

The first command validates; the second changes membership. The helper refuses
an existing kubelet identity and never resets a cluster. Its execution log is
protected at `/var/log/vmalpha-kubeadm-join.log`. After a successful join, verify
and approve only the selected node's serving certificate from a control plane:

```sh
python3 /usr/share/vmalpha/features/approve-serving.py \
  --node node2.example.internal --address 192.168.10.12 --approve
```

The helper checks the registered address, CSR subject/SANs, requester, usages and
signature, and preserves the inspected CSR version during approval. Revoke the
consumed bootstrap token and remove transferred join material after verifying
the node. A worker uses `--role worker` and must not receive control-plane
certificate or encryption-provider material. Control-plane nodes remain tainted
by kubeadm by default; enabling ordinary workloads is a separate explicit
converged-role choice, not a side effect of membership.

At each count (2, then 3, then 4), enroll into the existing cluster using
short-lived protected join material. For a control-plane member, supply the
same encryption-provider configuration through a protected transfer; do not
create a different encryption key on each API server. Verify the kubelet
serving CSR identity and SANs before approving it.

Confirm the expected node count and unique identity; verify system pods,
cross-node DNS and network access. Add only explicitly selected new Ceph disks.
Verify Ceph host failure domains and pool replication before claiming that
storage tolerates a host loss. Single-host OSD replication is not equivalent
to host-level redundancy.

Move the existing test StatefulSet to another eligible node through an orderly
shutdown, verify RWO attachment is released/reacquired, and compare the existing
file checksum. Do not force detach a volume while its previous writer may still
be active. Repeat real I/O and verify node-specific metrics, collector failures
and historical samples. Record screenshots and logs before the next expansion.

## Maintenance, removal and rejoin

Maintenance must actually stop new placement and safely evacuate or shut down
workloads. A database label alone is not a maintenance implementation. Respect
PodDisruptionBudgets and protect local-only data. Never force-remove a live
etcd member or OSD merely to make a dashboard green.

Before removal, verify remaining quorum/capacity and replicated data placement;
complete workload evacuation and storage recovery. Control-plane, worker and
Ceph membership removal are separate operations. Rejoin requires a reviewed
identity/state cleanup and fresh limited-lifetime enrollment material. Exact
product removal/rejoin commands are still under implementation and validation;
this document intentionally does not present untested destructive commands.

## Verified recovery behavior and remaining work

The one-node integration fixture preserved CSI PVC data, native RBD/NFS/iSCSI
VM disk checksums and Prometheus history after recovery from a forced reset and
then a successful graceful host reboot. The initial reboot hung when Ceph
stopped before CSI-mounted workloads were released. The fix configures kubelet
shutdown grace (90 seconds, including 30 seconds for critical pods), critical
CSI priorities and service ordering so container shutdown precedes Ceph stop.
Recovery may take several minutes while Ceph and CSI leadership/attachments
settle; a temporary unavailable collector must not be shown as zero utilization.

The existing RWO StatefulSet was moved from node 1 to node 2. Its PVC/PV identity
and data checksum remained unchanged, and cluster DNS worked from the new pod.
Both API servers also returned identical existing encrypted CSI secret data.
The new Ceph host contributes one explicitly selected blank 20 GiB disk; all
four OSDs recovered healthy after expansion. The three test pools now use a
host-level CRUSH rule with two replicas. Two monitors/etcd members remain a
transitional topology requiring both members for quorum.

The four-node worker also passed graceful reboot recovery with all nodes Ready,
Ceph healthy and all PVC data markers intact. Earlier two-node results above
record the transitional stage; they are superseded by the four-node topology.
Cross-node drain, member removal/rejoin, isolated etcd restore, physical
host-loss recovery and five-node operation have not passed acceptance yet. See ../ACCEPTANCE.md for the current test boundary.
