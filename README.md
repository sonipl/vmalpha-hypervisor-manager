# VM Alpha Hypervisor and Manager

Source snapshot of the VM Alpha Hypervisor and central Manager development projects.

## Source layout

- `hypervisor/`: Rocky Linux installer/media build, KVM/libvirt Host Client, API brokers, storage/network integration, Kubernetes/Ceph helpers, monitoring, tests and documentation.
- `manager/`: Go API, React frontend, native host integration, tests, Dockerfile and development appliance build/first-boot scripts.

Start with [Hypervisor documentation](hypervisor/README.md), [operator checks](hypervisor/docs/OPERATIONS.md), [cluster expansion](hypervisor/docs/EXPANSION.md), [native Manager deployment](manager/docs/NATIVE-DEVELOPMENT.md) and [Manager appliance build](manager/appliance/README.md).

## Building

Hypervisor `build.sh` requires its documented isolated Linux build tree, verified Rocky base media and a signed feature payload. These downloaded inputs and signing credentials are not included in Git. Before rendering/building, supply `VMALPHA_ADMIN_PASSWORD_HASH` through the build environment. It must be an operator-generated SHA-512 crypt hash. The original fixed lab password hash has been removed from this public snapshot. Protect rendered kickstart/build outputs, which contain the supplied hash.

The Manager Dockerfile builds its Go API and frontend. PostgreSQL and deployment configuration are supplied separately. The appliance directory documents the separate QCOW2 construction and first-boot process. Use the committed dependency lockfiles and source-specific test commands.

## Validation boundary

Historical local evidence records successful interactive ISO creation, BIOS/UEFI installed-platform checks and nested guest HTTPS tests, plus four-node development expansion and data persistence after a worker reboot. These are not a claim that this source snapshot has passed a complete release rebuild or that all Manager appliance features are finished. Consult component acceptance documents; older narrative status may predate individual test results.

No production HA or VMware feature-equivalence claim is made. The Manager appliance and full end-to-end release acceptance remain development work.

## Publication contents

This repository includes application source, build scripts, tests, manifests, vendored browser assets and their existing license notices. It excludes installed dependencies, generated test bundles, VM/ISO images, compiled binaries, live lab configurations, credentials and build caches. Provision runtime secrets and host enrollment through your own protected configuration. No new license is assigned to project-owned code by this import; existing third-party licenses and notices remain applicable.
