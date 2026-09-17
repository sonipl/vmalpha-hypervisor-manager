#!/bin/bash
set -euo pipefail
export LC_ALL=C
SOURCE=$(cd -- "$(dirname -- "$0")" && pwd)
BUILD=$(cd -- "$SOURCE/.." && pwd)
ISO="$BUILD/output/vm-alpha-hypervisor-1.0-x86_64-online.iso"
VERIFY="$BUILD/work/verify"
mkdir -p "$VERIFY"
sha256sum -c "$ISO.sha256"
xorriso -osirrox on -indev "$ISO" \
  -extract /vmalpha/vmalpha.ks "$VERIFY/vmalpha.ks" \
  -extract /EFI/BOOT/grub.cfg "$VERIFY/grub.cfg" \
  -extract /isolinux/isolinux.cfg "$VERIFY/isolinux.cfg" \
  -extract /images/efiboot.img "$VERIFY/efiboot.img" -end
cmp "$VERIFY/vmalpha.ks" "$BUILD/work/rendered/vmalpha.ks"
cmp "$VERIFY/grub.cfg" "$BUILD/work/rendered/grub.cfg"
cmp "$VERIFY/isolinux.cfg" "$BUILD/work/rendered/isolinux.cfg"
mtype -i "$VERIFY/efiboot.img" ::/EFI/BOOT/grub.cfg > "$VERIFY/embedded-grub.cfg"
cmp "$VERIFY/embedded-grub.cfg" "$BUILD/work/rendered/grub.cfg"
python3 - "$ISO" "$BUILD/logs/boot-report.txt" "$VERIFY/catalog-efi.img" <<'PY'
import pathlib,sys
line=next(l for l in pathlib.Path(sys.argv[2]).read_text().splitlines() if l.startswith('El Torito boot img :') and 'UEFI' in l)
size,lba=map(int,line.split()[-2:])
with open(sys.argv[1],'rb') as iso:
    iso.seek(lba*2048)
    image=iso.read(size*512)
if len(image)!=size*512: raise SystemExit('EFI image exceeds ISO bounds')
pathlib.Path(sys.argv[3]).write_bytes(image)
PY
mtype -i "$VERIFY/catalog-efi.img" ::/EFI/BOOT/grub.cfg > "$VERIFY/catalog-grub.cfg"
cmp "$VERIFY/catalog-grub.cfg" "$BUILD/work/rendered/grub.cfg"
grub2-script-check "$VERIFY/grub.cfg"
PYTHONPATH="$BUILD/work/validator/usr/lib/python3.9/site-packages" \
 /usr/libexec/platform-python "$BUILD/work/validator/usr/bin/ksvalidator" -v RHEL9 "$VERIFY/vmalpha.ks"
grep -q 'El Torito boot img :.*BIOS' "$BUILD/logs/boot-report.txt"
grep -q 'El Torito boot img :.*UEFI' "$BUILD/logs/boot-report.txt"
python3 - "$VERIFY/vmalpha.ks" <<'PY'
import base64,io,pathlib,subprocess,sys,tarfile
ks=pathlib.Path(sys.argv[1]).read_text()
post=ks.split('%post --erroronfail --log=/root/vmalpha-install.log\n')[1].rsplit('%end',1)[0]
subprocess.run(['bash','-n'],input=post,text=True,check=True)
data=ks.split("base64 -d <<'VMALPHA_PAYLOAD' | tar -xz -C /usr/share/vmalpha\n")[1].split('\nVMALPHA_PAYLOAD')[0]
with tarfile.open(fileobj=io.BytesIO(base64.b64decode(data))) as tf:
    for entry in tf:
        p=pathlib.PurePosixPath(entry.name)
        if p.is_absolute() or '..' in p.parts or entry.issym() or entry.islnk():
            raise SystemExit('Unsafe payload entry: '+entry.name)
    print('Embedded payload archive paths validated.')
print('Post-install shell syntax validated.')
PY
echo 'PASS: ISO SHA256, embedded files, catalog-addressed EFI FAT menu, BIOS/UEFI catalog, GRUB and RHEL9 kickstart syntax.'
echo 'NOT TESTED: actual firmware boot, installation, Cockpit authentication or nested guest lifecycle.'
