# Third-party notices and acknowledgments

## Rocky Linux base operating system

VM Alpha Hypervisor and the Manager appliance build use Rocky Linux® base media.
We acknowledge the Rocky Linux project authors, contributors and the Rocky
Enterprise Software Foundation for their work. VM Alpha's custom installer,
branding and management software are independent modifications. This project
does not represent itself as an official Rocky Linux release or claim upstream
affiliation, sponsorship, endorsement or support for its modifications.

Rocky Linux® is a registered trademark of the Rocky Enterprise Software Foundation.

- Project: https://rockylinux.org/
- Licensing and Rocky-developed component license: https://rockylinux.org/legal/licensing
- Trademark guidelines: https://rockylinux.org/legal/trademarks
- Package sources: https://git.rockylinux.org/

Rocky-developed elements and other bundled upstream components do not all share
one blanket license. Preserve the copyright, license texts and notices supplied
with each component. This acknowledgment does not relicense those components
or replace their license terms, including applicable source-distribution duties.
The licenses of VM Alpha's own code are not determined by the base OS license.

On an installed Rocky-based host, inspect package license metadata with:

```sh
rpm -qa --qf '%{NAME} - %{LICENSE}\n'
rpm -q --license <package-name>
```

Retain the upstream license files installed with packages, normally under
`/usr/share/licenses/`, and applicable source/provenance records when preparing
ISO or appliance releases. Review the exact shipped package set and any patches
for corresponding-source requirements; this document is not a complete SBOM
or certification of release compliance.

## Included third-party assets

Existing license files remain with their respective source and assets:

- noVNC: `hypervisor/hostclient/vendor/novnc/LICENSE.txt` and accompanying author notices.
- pako: `hypervisor/hostclient/vendor/novnc/vendor/pako/LICENSE`.
- Metropolis fonts: `hypervisor/hostclient/fonts/LICENSE.txt`.
- Flannel manifests: `hypervisor/features/manifests/flannel-LICENSE`.
- Ceph CSI operator manifests: `hypervisor/features/manifests/ceph-csi-operator-LICENSE`.

We also acknowledge the upstream Linux, KVM, QEMU, libvirt, Cockpit, Kubernetes,
containerd, Ceph, Prometheus, PostgreSQL, nginx, Go and JavaScript ecosystem
projects used by the source or build. The applicable versions, licenses and
notices are those accompanying each actual dependency. Dependency manifests
and lockfiles identify the source project's selected dependencies; dependency
names here do not imply upstream endorsement.
