#!/bin/bash
# Destructive cleanup only for this new, uninitialized appliance builder.
set -euo pipefail
test "$(id -u)" = 0
test "$(hostname)" = vmalpha-manager-builder
test -f /var/lib/vmalpha-manager-build-ready
test ! -e /var/lib/pgsql/data/PG_VERSION
test ! -e /var/lib/vmalpha-manager/initialized
test ! -e /etc/novasphere/config.yaml
test ! -e /etc/pki/vmalpha-manager/server.key
systemctl is-enabled vmalpha-manager-firstboot.service >/dev/null
unset HISTFILE
dnf clean all
usermod --password '!' root
usermod --password '!' rocky
rm -f /root/.ssh/authorized_keys /home/rocky/.ssh/authorized_keys
rm -f /root/.bash_history /home/rocky/.bash_history /etc/shadow- /etc/gshadow-
rm -f /etc/ssh/ssh_host_* /var/lib/systemd/random-seed
cloud-init clean --logs --seed --machine-id
rm -f /etc/NetworkManager/system-connections/cloud-init-*.nmconnection
rm -f /var/lib/NetworkManager/*.lease
hostnamectl set-hostname localhost.localdomain
printf '127.0.0.1 localhost localhost.localdomain\n::1 localhost localhost.localdomain\n' >/etc/hosts
rm -rf /tmp/vmalpha-package
rm -f /tmp/manager-package.tar.gz /tmp/vmalpha-manager-firstboot.service
rm -f /var/lib/vmalpha-manager-build-ready
find /var/log -type f -exec truncate -s 0 {} +
# Zero free blocks before standalone compressed export, including deleted builder data.
set +e
dd if=/dev/zero of=/var/tmp/vmalpha-zero bs=16M status=none
zero_status=$?
set -e
test "$zero_status" = 0 || test "$(df --output=avail / | tail -1 | tr -d ' ')" -lt 16384
rm -f /var/tmp/vmalpha-zero
sync
systemctl poweroff
