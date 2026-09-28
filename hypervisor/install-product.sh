#!/bin/bash
set -euo pipefail
SOURCE=$(cd -- "$(dirname -- "$0")" && pwd)
install -d /usr/share/cockpit/hostclient /usr/share/vmalpha /etc/cockpit /etc/sudoers.d
install -d -m 2755 -o root -g systemd-journal /var/log/journal
install -d /etc/systemd/journald.conf.d
printf '%s\n' '[Journal]' 'Storage=persistent' 'SystemMaxUse=256M' 'RuntimeMaxUse=64M' 'MaxRetentionSec=7day' > /etc/systemd/journald.conf.d/vmalpha.conf
cp -a "$SOURCE/hostclient/." /usr/share/cockpit/hostclient/
chown -R root:root /usr/share/cockpit/hostclient
# The kickstart extracts product sources directly into this destination.
# Preserve permissions there; copy only when running from a separate source tree.
for module in vmalpha_distributed_network.py vmalpha_monitoring.py vmalpha_san.py vmalpha_rbd.py vmalpha_storage.py vmalpha_containers.py components.json vmalpha_auth.py vmalpha_security.py vmalpha_datastore.py; do
 if [ "$SOURCE/$module" -ef "/usr/share/vmalpha/$module" ]; then
  chown root:root "/usr/share/vmalpha/$module"
  chmod 0644 "/usr/share/vmalpha/$module"
 else
  install -m 0644 "$SOURCE/$module" "/usr/share/vmalpha/$module"
 fi
done
install -m 0755 "$SOURCE/vmalpha-api.py" /usr/libexec/vmalpha-api
install -m 0755 "$SOURCE/vmalpha-vnc.py" /usr/libexec/vmalpha-vnc
install -m 0755 "$SOURCE/vmalpha-console.py" /usr/libexec/vmalpha-console
install -m 0755 "$SOURCE/storage-ready.py" /usr/libexec/vmalpha-storage-ready
install -m 0644 "$SOURCE/vmalpha-storage-ready.service" /etc/systemd/system/vmalpha-storage-ready.service
install -d /etc/systemd/system/virtqemud.service.d
printf '%s\n' '[Unit]' 'Wants=vmalpha-storage-ready.service' 'After=vmalpha-storage-ready.service' > /etc/systemd/system/virtqemud.service.d/vmalpha-storage.conf
getent group vmalpha >/dev/null || groupadd --system vmalpha
printf '%s\n' '%vmalpha ALL=(root) NOPASSWD: /usr/libexec/vmalpha-api, /usr/libexec/vmalpha-console *, /usr/libexec/vmalpha-vnc *' '%wheel ALL=(root) NOPASSWD: /usr/libexec/vmalpha-api, /usr/libexec/vmalpha-console *, /usr/libexec/vmalpha-vnc *' > /etc/sudoers.d/vmalpha-hostclient
chmod 0440 /etc/sudoers.d/vmalpha-hostclient
visudo -cf /etc/sudoers.d/vmalpha-hostclient
cat > /etc/cockpit/cockpit.conf <<'CONF'
[WebService]
LoginTitle = VM Alpha Hypervisor
LoginTo = false
AllowMultiHost = false
Shell = /hostclient/index.html
[Session]
Banner = /etc/issue
IdleTimeout = 15
CONF
# Establish a managed PAM profile on fresh minimal installations, retaining authselect backup.
if ! authselect current >/dev/null 2>&1; then
 authselect select minimal with-faillock --force
fi
# Keep ID, ID_LIKE, VERSION_ID, PLATFORM_ID and RPM repository identities.
python3 - <<'PY'
from pathlib import Path
p=Path('/etc/os-release');s=p.read_text();out=[]
for line in s.splitlines():
 key=line.split('=',1)[0]
 if key=='NAME':line='NAME="VM Alpha Linux"'
 if key=='VERSION':line='VERSION="1.0"'
 if key=='LOGO':line='LOGO="vmalpha"'
 if key=='PRETTY_NAME':line='PRETTY_NAME="VM Alpha Linux 1.0"'
 out.append(line)
# Replace the symlink with an independent presentation identity; preserve upstream file.
if p.is_symlink():p.unlink()
p.write_text('\n'.join(out)+'\n')
PY
printf '%s\n' 'VM Alpha Hypervisor 1.0 (VM Alpha Linux 1.0)' > /etc/vmalpha-release
printf '%s\n' 'VM Alpha Hypervisor 1.0' 'Authorized system access only.' > /etc/issue
printf '%s\n' 'VM Alpha Hypervisor 1.0 on VM Alpha Linux 1.0' 'Web management: https://<host-address>:9090' > /etc/motd
install -d /usr/share/vmalpha/login
test -f /usr/share/vmalpha/login/upstream-login.html || cp /usr/share/cockpit/static/login.html /usr/share/vmalpha/login/upstream-login.html
cp /usr/share/vmalpha/login/upstream-login.html /usr/share/vmalpha/login/login.html
install -m 0644 "$SOURCE/branding/branding.css" /usr/share/cockpit/static/vmalpha-branding.css
install -m 0644 "$SOURCE/branding/brand.js" /usr/share/cockpit/static/vmalpha-brand.js
sed -i 's@</head>@<script defer src="cockpit/static/vmalpha-brand.js"></script></head>@' /usr/share/vmalpha/login/login.html
sed -i 's@cockpit/static/branding.css@cockpit/static/vmalpha-branding.css?v=1.0@' /usr/share/vmalpha/login/login.html
cp /usr/share/vmalpha/login/login.html /usr/share/cockpit/static/login.html
# Product branding override uses supported Cockpit branding paths.
for dir in /etc/cockpit/branding /usr/share/cockpit/branding/default; do
 install -d "$dir"
 cp "$SOURCE/branding/branding.css" "$dir/branding.css"
done
if test -f /etc/default/grub; then
 sed -i 's/^GRUB_DISTRIBUTOR=.*/GRUB_DISTRIBUTOR="VM Alpha Linux"/' /etc/default/grub
fi
# BLS titles are presentation only; kernels and upstream package IDs are preserved.
for entry in /boot/loader/entries/*.conf; do
 test ! -f "$entry" || sed -i 's/^title Rocky Linux/title VM Alpha Linux/' "$entry"
done
restorecon -RF /usr/share/cockpit/hostclient /usr/libexec/vmalpha-api /usr/libexec/vmalpha-console /usr/libexec/vmalpha-vnc /etc/cockpit /etc/sudoers.d/vmalpha-hostclient /etc/os-release /etc/vmalpha-release /usr/share/vmalpha /usr/share/cockpit/static/vmalpha-brand.js /usr/share/cockpit/static/vmalpha-branding.css

# Keep early installed-boot identity aligned when future kernels regenerate initramfs.
install -d /usr/lib/dracut/modules.d/99vmalpha
cat > /usr/lib/dracut/modules.d/99vmalpha/module-setup.sh <<'DRACUT'
#!/bin/bash
check() { return 0; }
depends() { return 0; }
install() {
 mkdir -p "$initdir/etc" "$initdir/usr/lib"
 rm -f "$initdir/etc/initrd-release" "$initdir/usr/lib/initrd-release"
 cp /etc/os-release "$initdir/etc/initrd-release"
 cp /etc/os-release "$initdir/usr/lib/initrd-release"
}
DRACUT
chmod 0755 /usr/lib/dracut/modules.d/99vmalpha/module-setup.sh
