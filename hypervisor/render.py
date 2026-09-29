#!/usr/bin/env python3
"""Render a self-contained interactive kickstart and boot menus."""
import base64
import os
import re
import io
import pathlib
import tarfile
import textwrap
import sys

password_hash = os.environ.get('VMALPHA_ADMIN_PASSWORD_HASH', '')
if not re.fullmatch(r'\$6\$[A-Za-z0-9./]{1,16}\$[A-Za-z0-9./]{86}', password_hash):
    raise SystemExit('Set VMALPHA_ADMIN_PASSWORD_HASH to an operator-supplied SHA-512 crypt hash')

source = pathlib.Path(__file__).resolve().parent
out = pathlib.Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
payload = io.BytesIO()
def product_member(member):
    if '__pycache__' in member.name.split('/') or member.name.endswith(('.pyc', '.pyo')):
        return None
    member.uid = member.gid = 0
    member.uname = member.gname = 'root'
    member.mode &= 0o777
    return member

with tarfile.open(fileobj=payload, mode='w:gz') as tf:
    for name in ('README.md', 'ACCEPTANCE.md', 'docs', 'storage-ready.py', 'vmalpha-storage-ready.service', 'monitoring', 'features', 'vmalpha_monitoring.py', 'vmalpha_san.py', 'vmalpha_rbd.py', 'vmalpha_storage.py', 'vmalpha_containers.py', 'components.json', 'vmalpha_auth.py', 'vmalpha_security.py', 'vmalpha_datastore.py', 'hostclient', 'branding', 'install-product.sh', 'vmalpha-api.py', 'vmalpha-ssh-gateway.py', 'vmalpha-console.py', 'vmalpha-vnc.py', 'firstboot.sh', 'vmalpha-firstboot.service', 'default-network.xml'):
        tf.add(source / name, arcname=name, filter=product_member)
encoded = '\n'.join(textwrap.wrap(base64.b64encode(payload.getvalue()).decode(), 76))
packages = (source / 'packages.txt').read_text().strip()
kickstart = '''# VMALPHA 1.0 / Rocky Linux 9.8: ONLINE, interactive install.
# Disk selection remains interactive. Initial UI administrator is requested by the owner.
graphical
lang en_US.UTF-8
keyboard us
timezone Etc/UTC --utc
network --bootproto=dhcp --device=link --activate
url --url=https://download.rockylinux.org/pub/rocky/9.8/BaseOS/x86_64/os/
repo --name=rocky98-appstream --baseurl=https://download.rockylinux.org/pub/rocky/9.8/AppStream/x86_64/os/
rootpw --lock
user --name=admin --groups=wheel --password=@@ADMIN_PASSWORD_HASH@@ --iscrypted
selinux --enforcing
firewall --enabled --service=ssh --service=cockpit
services --enabled=sshd,chronyd,cockpit.socket,NetworkManager,firewalld
firstboot --disable
eula --agreed
reboot --eject

%packages --exclude-weakdeps
@core
''' + packages + '''
%end

%post --nochroot --erroronfail --log=/mnt/sysroot/root/vmalpha-payload-copy.log
set -eu
test -f /run/install/repo/vmalpha/features/payload-manifest.json
install -d /mnt/sysroot/usr/share/vmalpha/features
cp -a /run/install/repo/vmalpha/features/. /mnt/sysroot/usr/share/vmalpha/features/
chown -R root:root /mnt/sysroot/usr/share/vmalpha/features
%end

%post --erroronfail --log=/root/vmalpha-install.log
set -eu
install -d -m 0755 /usr/share/vmalpha
base64 -d <<'VMALPHA_PAYLOAD' | tar -xz -C /usr/share/vmalpha
''' + encoded + '''
VMALPHA_PAYLOAD
bash /usr/share/vmalpha/install-product.sh
bash /usr/share/vmalpha/features/install-offline.sh /usr/share/vmalpha/features
bash /usr/share/vmalpha/monitoring/install.sh /usr/share/vmalpha/features --defer-start
# Regenerate installed boot images with the product identity.
dracut --regenerate-all --force
install -m 0755 /usr/share/vmalpha/firstboot.sh /usr/libexec/vmalpha-firstboot
install -m 0644 /usr/share/vmalpha/vmalpha-firstboot.service /etc/systemd/system/vmalpha-firstboot.service
# Rocky 9.8 presets modular daemons; monolithic libvirtd conflicts with them.
systemctl disable libvirtd.service libvirtd.socket libvirtd-ro.socket libvirtd-admin.socket
systemctl enable virtqemud.socket virtnetworkd.socket virtstoraged.socket virtproxyd.socket virtlogd.socket virtlockd.socket cockpit.socket vmalpha-firstboot.service
restorecon -RF /usr/share/vmalpha /usr/share/cockpit/hostclient /usr/libexec/vmalpha-firstboot /etc/vmalpha-release /etc/cockpit
%end
'''
(out / 'vmalpha.ks').write_text(kickstart.replace('@@ADMIN_PASSWORD_HASH@@', password_hash))
args = 'inst.stage2=hd:LABEL=VMALPHA-1-0 inst.ks=hd:LABEL=VMALPHA-1-0:/vmalpha/vmalpha.ks ip=dhcp'
(out / 'grub.cfg').write_text('''set default=0
set timeout=-1
insmod efi_gop
insmod efi_uga
insmod all_video
insmod gzio
insmod part_gpt
insmod ext2
search --no-floppy --set=root -l 'VMALPHA-1-0'
menuentry 'Install VM Alpha Hypervisor 1.0 - VM Alpha Linux (online, interactive)' {
    linuxefi /images/pxeboot/vmlinuz ''' + args + ''' quiet
    initrdefi /images/pxeboot/initrd.img
}
menuentry 'Test media and install VM Alpha Hypervisor (online, interactive)' {
    linuxefi /images/pxeboot/vmlinuz ''' + args + ''' rd.live.check quiet
    initrdefi /images/pxeboot/initrd.img
}
menuentry 'Rescue system' {
    linuxefi /images/pxeboot/vmlinuz inst.stage2=hd:LABEL=VMALPHA-1-0 inst.rescue
    initrdefi /images/pxeboot/initrd.img
}
''')
(out / 'isolinux.cfg').write_text('''default vesamenu.c32
prompt 0
timeout 0
menu title VM Alpha Hypervisor 1.0 - VM Alpha Linux
label vmalpha
  menu label Install VM Alpha Hypervisor (online, interactive)
  menu default
  kernel vmlinuz
  append initrd=initrd.img ''' + args + ''' quiet
label check
  menu label Test media and install VM Alpha Hypervisor
  kernel vmlinuz
  append initrd=initrd.img ''' + args + ''' rd.live.check quiet
label rescue
  menu label Rescue system
  kernel vmlinuz
  append initrd=initrd.img inst.stage2=hd:LABEL=VMALPHA-1-0 inst.rescue
''')
print('Rendered kickstart and BIOS/UEFI menus:', out)
