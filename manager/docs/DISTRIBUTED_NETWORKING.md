# Distributed network workflow

Networking now has switch/bridge and VLAN port-group create/edit/delete controls. The Manager persists the desired object and a per-host result; it never changes host networking when displaying or saving a review.

1. Select enrolled hosts, bridge/uplink, MTU, and port groups (0 untagged, 1–4094 tagged).
2. Review asks every host's `network.distributed.preview` operation for capabilities and a fingerprint. Any unavailable/refusing host blocks the entire apply.
3. Explicit confirmation consumes an owner-bound review valid for five minutes. A database revision lock prevents conflicting/replayed applies.
4. Sequential fanout invokes `network.distributed.apply` with its host fingerprint. The executor must revalidate atomically. First failure halts later hosts; partial results remain visible and require reconciliation. No automatic retry or silent rollback occurs.

Bridge/uplink/target membership edits are excluded: these require a separately planned management/VM migration. Deletion keeps a tombstone for audit. Target enrollment is checked again at execution. RBAC requires networking create/update/delete permissions for review/apply; read-only roles cannot mutate.

## Broker contract

Request arguments: `id` UUID, `action` create/update/delete, `spec` containing name/bridge/uplink/mtu/hosts/port_groups. Apply additionally requires `expected_fingerprint`. Preview response must contain `supported:true` and nonempty `fingerprint`; apply must contain `applied:true`. Host executor must protect management routes, addressed/enslaved uplinks, existing non-owned bridges, and guest attachments; it must preserve rollback state and use fixed commands, never shell interpolation.

The Manager deliberately fails closed on older brokers. **Installing this Manager feature alone does not supply a compatible host executor or prove any live bridge operation.** Deploy and test the matching broker in an isolated environment before using a spare physical uplink. No production network changes are part of the software deployment.

## Deployment

Build backend and frontend from the same revision, run database migrations for DistributedNetwork and DistributedNetworkReview, then deploy the normal Manager service/UI artifact. No first-boot or Ansible hook invokes apply: future builds ship controls and capabilities, while actual network mutation always requires the reviewed operator action.

Validation: backend handler/route tests, strict TypeScript checking, refused broker capability, VLAN/name constraints, expired/replayed review rejection, and explicit confirmation requirements. Live switch/port-group mutation and recovery remain a separate isolated integration gate.

## Supplied host implementation and discovery

The matching implementation is now `hypervisor/vmalpha_distributed_network.py`; code-only Ansible deployment and detailed topology/recovery notes are in `deployment/distributed-network/`. `POST /distributed-networks/discover` accepts `hosts` and returns per-host read-only interface, bridge membership, state and eligibility data. The form pre-populates a sole common eligible uplink and displays differences. Existing unmanaged bridges stay read-only rather than being silently taken over.

The Linux bridge implementation creates stable VLAN access bridges for tagged port groups and returns their actual interface names. Update/delete currently require no guest or management use on all affected interfaces. Rollback is best-effort per host and reported explicitly; interrupted or failed rollback blocks subsequent operations. No live network integration test is claimed by source tests.
