#!/bin/bash
set -euo pipefail
# Runs only inside the installed VMALPHA host, never on the build server.
test -f /etc/vmalpha-release
test ! -f /var/lib/vmalpha/initialized
systemctl start virtqemud.socket virtnetworkd.socket virtstoraged.socket
for attempt in $(seq 1 30); do
  virsh -c qemu:///system list >/dev/null 2>&1 && break
  sleep 1
done
virsh -c qemu:///system list >/dev/null
if ! virsh -c qemu:///system net-info default >/dev/null 2>&1; then
  virsh -c qemu:///system net-define /usr/share/vmalpha/default-network.xml
fi
virsh -c qemu:///system net-autostart default
if ! virsh -c qemu:///system net-info default | grep -qE '^Active:.*yes'; then
  virsh -c qemu:///system net-start default
fi
install -d -m 0755 /var/lib/libvirt/images
restorecon -RF /var/lib/libvirt/images
if ! virsh -c qemu:///system pool-info default >/dev/null 2>&1; then
  virsh -c qemu:///system pool-define-as default dir --target /var/lib/libvirt/images
fi
virsh -c qemu:///system pool-autostart default
if ! virsh -c qemu:///system pool-info default | grep -qE '^State:.*running'; then
  virsh -c qemu:///system pool-start default
fi
install -d -m 0755 /var/lib/vmalpha
touch /var/lib/vmalpha/initialized
