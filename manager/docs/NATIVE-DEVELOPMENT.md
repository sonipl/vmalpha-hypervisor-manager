# Native Manager development deployment

This is a development deployment guide, not the reusable QCOW2 release guide.
The full acceptance matrix remains open in FEATURE-ACCEPTANCE.md.

The API requires PostgreSQL, an operator-supplied JWT secret of at least 32 bytes,
and NOVA_BOOTSTRAP_PASSWORD (12–72 bytes) for a new database. Existing databases
are not reset. Keep credentials outside the source tree and image. Supply config
at /etc/novasphere/config.yaml or in the service working directory. Sensitive
values may be supplied using NOVA_DATABASE_PASSWORD and NOVA_AUTH_JWT_SECRET.

Enroll hosts using the native_hosts configuration map. Each entry has address
(host:port), user, private_key_file, and known_hosts_file. Only explicitly enrolled
hosts are reachable through the native API. Verify host public keys through an
already trusted management channel before writing known_hosts. Private keys must
be regular files without group/other access. The service identity needs access
to those files; the container runs as an unprivileged user.

On each hypervisor, provision a dedicated SSH key restricted to the Manager's
source address and the forced command /usr/libexec/vmalpha-ssh-gateway, with forwarding,
PTY and user startup files disabled (OpenSSH restrict option). The test enrollment
uses such a key under root so the fixed broker can apply its existing privilege
checks; it permits only the fixed broker and a name-validated local VNC stream,
and does not expose a general remote shell. Do not reuse personal or Ceph
identities. Never include the private key, host enrollment or deployment config
in a reusable appliance image. Enrollment UI and rotation remain pending.

Native routes require Platform Admin. GET /api/v1/native/hosts lists configured
names; GET /api/v1/native/hosts/{name}/inventory returns live broker inventory.
POST /api/v1/native/hosts/{name}/operations accepts op and args plus a UUID
Idempotency-Key header. Current operations are vm-create, vm-action, vm-edit, vm-attach-disk, pool-create, volume-create,
network-create and network-action. A read-only files endpoint lists the enrolled
host guest datastore and refuses paths outside that root. Arguments follow the fixed Host Client broker contract. A durable
intent is saved before execution. The same actor/host/body/key replays the saved
task instead of repeating the operation. Keys cannot be reused for another body,
actor or host. GET /api/v1/native/tasks/{id} reads the saved outcome.

Running or Unknown after a disconnect requires host readback before any new
attempt. There is no automatic retry or completed reconciliation workflow yet.
The native UI remembers unresolved operation IDs across reloads. Task history
contains identifiers/status and a request digest, not request bodies or secrets.

Development checks: go test ./cmd/... ./internal/...; in web run npm ci,
npm run typecheck, npm run test:ui and npm run build. The Dockerfile builds the
API and frontend but does not package PostgreSQL or produce a QCOW2 appliance.

A Succeeded task confirms successful broker command execution. Guest shutdown
is asynchronous: confirm shut-off state in live inventory before a dependent
action. The UI explicitly calls this a shutdown request. VM create/start acceptance
also verifies the resulting domain and running state, beyond the task response.
