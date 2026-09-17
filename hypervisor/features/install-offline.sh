#!/bin/bash
set -euo pipefail
SOURCE=$(cd -- "$(dirname -- "$0")" && pwd)
PAYLOAD=${1:-/usr/share/vmalpha/features}
python3 "$SOURCE/verify-payload.py" "$PAYLOAD"
for key in "$PAYLOAD"/keys/RPM-GPG-KEY* "$PAYLOAD"/keys/ceph-release.asc; do
 rpm --import "$key"
done
dnf -y --disablerepo='*' --repofrompath="vmalpha-features,file://$PAYLOAD/rpms" \
 --setopt=vmalpha-features.gpgcheck=1 install \
 cephadm ceph-common podman nfs-utils iscsi-initiator-utils \
 device-mapper-multipath libvirt-daemon-driver-storage-rbd qemu-kvm-block-rbd \
 python3-pyyaml conntrack-tools socat iptables-nft
# Explicitly select the signed compatibility build even if the base installer
# already installed the upstream RBD driver through a libvirt metapackage.
shopt -s nullglob
rbd_packages=("$PAYLOAD"/rpms/libvirt-daemon-driver-storage-rbd-*vmalpha*.rpm)
[ "${#rbd_packages[@]}" -eq 1 ] || { echo 'Exactly one VM Alpha RBD driver is required'; exit 1; }
dnf -y --disablerepo='*' --repofrompath="vmalpha-features,file://$PAYLOAD/rpms" \
 --setopt=vmalpha-features.gpgcheck=1 --setopt=localpkg_gpgcheck=1 install "${rbd_packages[0]}"
bash "$SOURCE/install-runtime.sh" "$PAYLOAD"
cat > /etc/systemd/system/vmalpha-feature-images.service <<'UNIT'
[Unit]
Description=Load verified VM Alpha feature images
Requires=containerd.service
After=containerd.service
[Service]
Type=oneshot
ExecStart=/bin/bash /usr/share/vmalpha/features/load-images.sh
TimeoutStartSec=20min
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
UNIT
systemctl enable containerd.service vmalpha-feature-images.service
printf '%s\n' 'Offline feature packages and runtimes installed; archived images load on first boot.'
