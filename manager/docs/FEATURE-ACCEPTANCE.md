# VM Alpha Manager feature and release gates

Development inventory, 2026-09-16. No Manager appliance has been built or tested.
A route or database model is not evidence that the corresponding infrastructure
operation works. The recovered application primarily targets Kubernetes/KubeVirt;
the current Hypervisor requires native libvirt integration.

| Requested area | Recovered baseline | Required implementation and acceptance |
|---|---|---|
| Datacenters | No datacenter lifecycle identified in routed API | Hierarchical inventory, scope propagation, move/remove guards |
| Clusters | List/read/create routes | Real host membership, capacity, placement and maintenance constraints |
| Hosts | Inventory and a maintenance handler | Verified enrollment, live reconciliation, actual placement gate and safe evacuation; current maintenance only changes database state |
| VMs | CRUD and action routes | Native libvirt create/import/start/stop/reconfigure/console/disk/NIC/snapshot lifecycle; multi-host readback and failures |
| Networking | Network/firewall CRUD | Real bridge/network/VLAN/port/port-group operations, connectivity guards, persistent host configuration and conflict handling |
| Datastores | Local/NFS/Ceph backend routes | Native local/NFS/iSCSI/FC/RBD discovery and operations, credential protection, consumer guards, actual capacity |
| Datastore clusters | No verified implementation | Grouping, policy/capacity behavior and supported relocation semantics; no automated storage-placement claim without tests |
| Users/roles | JWT and role/tenant models | Live account/role revocation, scoped object checks, specific signing algorithm, session/console revocation, privilege tests on every route |
| Tasks/events | Audit/event feed exists | Durable operation state, IDs, real success/failure, retries/idempotency, crash recovery and redacted errors |
| Monitoring | Dashboard returns constant health/utilization | Authenticated real host/VM/cluster/storage series, missing/stale state, history, alert evaluation and scoped access; constants must be removed |
| Maintenance/migration | Unverified baseline actions | Quorum/capacity checks, placement block, real VM/workload movement and disk/network continuity; unsupported combinations explicit |
| HA/advanced cluster behavior | Not verified | Native equivalents with measured failure/recovery tests; never claim VMware proprietary feature parity from names alone |
| Backup/restore | Routes exist | Application DB/config/certificate backup, isolated restore, host reconnection and task reconciliation |
| Reusable QCOW2 | No new image | Unique machine/SSH/TLS identity, operator first-boot setup, clean storage, no fixture tokens/host keys/default live secrets, checksum and two-copy deployment |

## Security work identified by source review

Review setup, settings, AI and monitoring route groups: several mutations have
only the general authentication middleware and lack explicit RBAC checks.
Review object-level and tenant filtering independently of route-level roles.
Whitelist host/VM sort fields and direction instead of passing request strings
into SQL ordering. Refresh user and role state when authorizing requests;
revoking a role must affect an existing session. Restrict JWT signing methods.
Treat all recovered seed/configuration code as development input; do not embed
sample identities or passwords into the reusable appliance.

## Acceptance sequence

1. Build and test the recovered baseline in isolation; pin supported dependencies.
2. Implement the native authenticated host adapter and durable operation model.
3. Exercise each requested feature against isolated hosts, including denied,
   failed and interrupted operations. A database-only change cannot pass.
4. Produce a clean appliance and deploy two separate copies to prove identity
   regeneration. Configure network/TLS/operator access without editing an image.
5. Enroll multiple isolated hosts, perform real lifecycle/network/storage actions,
   verify authorization and history, then test backup/restore and reboot.
6. Deliver a feature matrix with evidence and explicit unsupported cases alongside
   QCOW2, checksum, installation guide and operator/recovery instructions.

The live hypervisor expansion and exact ISO acceptance remain prerequisites for
claiming the complete requested solution. This matrix is a backlog, not a
completion report or an inventory-only substitute for the Manager deliverable.
# Authorization development check — 2026-09-17 UTC

The authentication middleware now pins HS256, issuer, expiration and subject/user
identity. Every request reloads the current active account and role, so account
disablement and role/tenant changes take effect without waiting for token expiry.
Malformed RBAC context fails closed. Focused Go tests passed for stale-role
demotion, current tenant/username, disabled/missing accounts, store failure,
missing role, wrong algorithm/issuer/subject, absent/expired timestamps and future
issuance. This is a source-level security milestone, not Manager appliance or
end-to-end authorization acceptance; resource-level tenancy, custom permissions,
session revocation and the remaining central operations still require completion.

Host/VM inventory sorting now accepts only known model fields and `asc`/`desc`,
uses quoted GORM column clauses, and supports the dashboard's `-created_at`
syntax. Invalid fields, SQL expressions and injected direction strings return
400 before synchronization or database access. Focused endpoint regression tests
passed. `go test ./...` passed after both changes; most recovered packages have
no pre-existing tests, so this is not full behavior or deployment acceptance.

Twenty-four previously unguarded AI/setup/LDAP/monitoring routes now have explicit
permission checks. Sensitive operations currently require the current Platform
Admin; no delegated permission has been added for those resources. A route-level
test verifies denial before handler/database access for 21 sensitive routes and
five non-administrator roles. Resource-specific delegated administration remains
a future acceptance gate, not an implied capability of these guards.

The Go dependencies were updated and the build toolchain pinned to Go 1.27.1.
All Go package tests pass afterward. The Linux-targeted govulncheck scan reports
zero reachable vulnerabilities and zero vulnerable imported packages. The sole
module-level advisory is GO-2026-5932 for the unmaintained OpenPGP package inside
golang.org/x/crypto; the Manager imports bcrypt, not OpenPGP, and the scanner
confirms the affected package is not imported. No fixed version is listed. The earlier recovered
dependency graph had four reachable findings. These results do not replace
integration tests, database tenancy checks, or the appliance acceptance matrix.

### Dashboard data integrity — source verification, 2026-09-17
Removed hard-coded utilization, random history, fabricated VM lists/insights,
and unconditional healthy status from the dashboard. API now counts actual VM
states (including migrating, paused and provisioning), returns HTTP 503 when
inventory queries fail, and explicitly reports unknown health/null utilization
until live monitoring is connected. UI consumes the API's nested inventory and
`data` pagination shape and renders loading/error/empty states without samples.
Backend regression tests pass, including unavailable database and absent telemetry.
Vite production build passes. Full TypeScript check still fails in other existing
pages/components/API types; its log is retained as a release blocker. No live
Manager deployment or appliance acceptance is implied by these source checks.
Monitoring UI also removes random event/latency charts and fabricated alerts;
actual stored rules and audit events use the API schema, with explicit errors.
Storage methods were moved from the erroneous authAPI group to storageAPI, with
a shared StorageBackend type. DataTable accepts typed object rows without an
index signature; VM action/status filters now match backend enum capitalization.
React Router DOM pinned to 7.18.4: npm production dependency audit reports zero
findings. Full frontend type checking remains blocked by other existing pages;
build/audit success is not functional appliance acceptance.

### Frontend integrity gate follow-up
Full `tsc --noEmit` now passes after correcting pagination, typed table rows,
API method grouping, status enums, and actual response fields. Placeholder users,
roles, networks, rules, VM detail data, snapshots, utilization and fake connected
console output were removed from these pages. Unimplemented controls are no
longer presented as working operations. These still require implementation for
the full Manager scope; the cleanup is not an inventory-only release approval.
Three dashboard rendering regression tests pass for loading/unavailable telemetry,
actual nested count/pagination data, and hiding stale data after failed refresh.
Reproduction: npm run typecheck; npm run test:ui; npm run build. Production npm
audit is zero findings; backend full tests pass. Full live integration, browser
interaction coverage, central native operations and QCOW2 appliance remain open.

### Container packaging follow-up
The Docker builder now uses Go 1.27.1 with automatic toolchain downloads disabled,
Node 24, a lockfile-based npm install, and mandatory TypeScript checking before
bundling. Runtime uses Alpine 3.24. Removed a COPY of a nonexistent example
configuration; deployment configuration must be mounted at runtime. Docker build
context excludes dependency trees, generated bundles, deployment configuration,
keys and disk/media images. Local production frontend rebuild passed. Container
build is underway; this does not establish appliance or deployment acceptance.

The first Docker packaging build passed (development image only, not deployed).
Subsequent bootstrap hardening removes the shipped administrator password and
its startup log disclosure. An empty database requires an operator-supplied
NOVA_BOOTSTRAP_PASSWORD (12–72 bytes); an existing user database is left intact.
Tenant/role/user bootstrap writes now use one transaction. Tests cover missing,
short and oversized passwords, preserving existing users and rollback on failure.
The image must be rebuilt after this source change before using it for acceptance.

The container rebuild including bootstrap hardening passed. A temporary
read-only container with networking disabled confirmed non-root runtime, API
executable and frontend assets, with no baked deployment config. This is
packaging acceptance only; live database and native-host operations are pending.

### Native transport source gate
Added a native-host SSH transport for the Hypervisor fixed JSON broker. It
requires an explicit known-hosts file and private key with private permissions,
uses a fixed command with JSON stdin, bounds requests/responses, applies a
context deadline, closes connections on cancellation, and does not automatically
retry mutations. Broker stderr/errors are not returned verbatim. Tests pass for
changed host keys, private-key file permissions, malformed/rejected responses,
and the response size limit including io.Copy's writer fast path. Enrollment,
route authorization, durable operations and live multi-host integration are
still pending; this package is not yet wired into the Manager API.

### First live native operation acceptance
A dedicated forced-command SSH identity was enrolled on an isolated fixture,
restricted to the test controller source address. Manager authentication,
native inventory, create/import and start were exercised through the actual
HTTP API against that host. Live readback showed exactly one running new guest.
Replaying the same create idempotency key returned the existing durable task,
without repeating the mutation. Both task records read back as Succeeded.

The native API is currently restricted to Platform Admin and hosts enrolled in
operator configuration. DB intent is stored before execution. A remote error or
disconnect records Unknown; mutations are never automatically retried. Tests
cover DB failure before host execution, uncertain execution, duplicate replay,
and non-admin route denial. Negotiation now uses the enrolled host key algorithms;
the live test caught and fixed an initial SSH key-selection mismatch.

A native-host UI now exposes live inventory, import, start/shutdown/resume and
saved operation status. Pending/unknown operation IDs persist across reloads,
preventing an accidental new request until the outcome is checked. Four native
page rendering tests and three dashboard tests pass; TypeScript checking passes.
This is still development acceptance: browser sign-in/action coverage, multi-host
enrollment UX, full lifecycle/network/storage, reconciliation, tenant-level
permissions, HA, backup/restore and the reusable QCOW2 remain incomplete.

Manager restart acceptance passed: existing credentials and both native task
records survived restart; replay still returned the saved task, and live inventory
contained exactly one running guest. Startup validation now rejects an absent or
short JWT signing key before opening services. Operator deployment configuration
must supply at least 32 bytes of secret material, and first startup also requires
the bootstrap password. The test service is bound only to localhost.

Two-host native inventory acceptance also passed with distinct enrolled host
identities. Login-page branding and accessible labels were checked in the local
browser. The native workflow still has source/render tests and API end-to-end
acceptance, not browser end-to-end sign-in/action coverage.

### Live monitoring bridge
Authenticated fixed monitoring queries now pass through the enrolled native
broker. Host CPU history from two hosts and the Manager-created guest CPU history
passed with measured samples and healthy collectors. The central collector host
returned Ceph health 0 and four Ready Kubernetes nodes, also with measured history.
Querying those scopes on hosts without configured collectors correctly returned
unavailable collector state and no samples; this is not treated as healthy zero.
The UI offers host/VM/container/Ceph scopes, metric/range/series selection, and
explicit missing/stale states. Ten frontend rendering tests pass. TypeScript and
production build pass after the monitoring changes. Central alert management,
full enrollment workflow, delegated object-level permissions and QCOW2 packaging
remain open.
