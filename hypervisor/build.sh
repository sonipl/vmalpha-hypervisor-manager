#!/bin/bash
set -euo pipefail
export LC_ALL=C
SOURCE=$(cd -- "$(dirname -- "$0")" && pwd)
BUILD=$(cd -- "$SOURCE/.." && pwd)
case "$BUILD" in /repos/vmalpha-rocky9/builds/*) ;; *) echo 'Refusing build outside isolated build tree'; exit 1;; esac
BASE="$BUILD/inputs/Rocky-9.8-x86_64-boot.iso"
VERSION=$(tr -d '\n' < "$SOURCE/VERSION")
case "$VERSION" in ''|*[!0-9.]*|.*|*.) echo 'Invalid VM Alpha version'; exit 1;; esac
OUT="$BUILD/output/vm-alpha-hypervisor-${VERSION}-x86_64-online.iso"
PAYLOAD=$(realpath -e "$BUILD/features")
test -f "$PAYLOAD/payload-manifest.json"
python3 "$SOURCE/features/verify-payload.py" "$PAYLOAD"
test ! -e "$OUT" || { echo 'Output already exists; use a new versioned build directory'; exit 1; }
test "$(df -Pk "$BUILD" | awk 'NR==2 {print $4}')" -gt 10485760
for tool in xorriso python3 sha256sum implantisomd5 checkisomd5 mcopy grub2-script-check; do command -v "$tool"; done
python3 - "$BASE" "$BUILD/inputs/CHECKSUM" <<'PY'
import hashlib,pathlib,re,sys
iso=pathlib.Path(sys.argv[1]); manifest=pathlib.Path(sys.argv[2]).read_text()
expected=re.search(r'^SHA256 \(Rocky-9\.8-x86_64-boot\.iso\) = ([a-f0-9]{64})$',manifest,re.M)
if not expected: raise SystemExit('Official checksum entry missing')
h=hashlib.sha256()
with iso.open('rb') as f:
    for block in iter(lambda:f.read(1024*1024),b''): h.update(block)
if h.hexdigest()!=expected.group(1): raise SystemExit('Base ISO checksum mismatch')
print('Base ISO SHA256 verified:',h.hexdigest())
PY
(cd "$SOURCE/product" && find . -print0 | cpio --null -o -H newc --owner=0:0 | gzip -9) > "$BUILD/work/product.img"
python3 "$SOURCE/render.py" "$BUILD/work/rendered"
grub2-script-check "$BUILD/work/rendered/grub.cfg"
# UEFI uses a second grub.cfg inside the FAT boot image. Update both copies.
xorriso -osirrox on -indev "$BASE" \
  -extract /images/efiboot.img "$BUILD/work/rendered/efiboot.img" -end
chmod u+w "$BUILD/work/rendered/efiboot.img"
mcopy -o -i "$BUILD/work/rendered/efiboot.img" "$BUILD/work/rendered/grub.cfg" ::/EFI/BOOT/grub.cfg
# Append a small initramfs overlay for early-boot presentation identity.
# Linux supports concatenated compressed cpio archives; the upstream kernel is unchanged.
xorriso -osirrox on -indev "$BASE" -extract /images/pxeboot/initrd.img "$BUILD/work/base-initrd.img" -end
mkdir -p "$BUILD/work/initrd-brand/etc" "$BUILD/work/initrd-brand/usr/lib"
cp "$SOURCE/product/etc/os-release" "$BUILD/work/initrd-brand/etc/initrd-release"
cp "$SOURCE/product/etc/os-release" "$BUILD/work/initrd-brand/usr/lib/initrd-release"
(cd "$BUILD/work/initrd-brand" && find . -print0 | cpio --null -o -H newc --owner=0:0 | gzip -9) > "$BUILD/work/initrd-brand.gz"
cat "$BUILD/work/base-initrd.img" "$BUILD/work/initrd-brand.gz" > "$BUILD/work/rendered/initrd.img"
# The verified Rocky boot ISO has no .treeinfo; installation uses online repos.
# ISO boot replay preserves the original BIOS/UEFI boot records and hybrid layout.
# No loop mounts, host services, package installs, or sudo are used.
xorriso -indev "$BASE" -outdev "$OUT" \
  -map "$BUILD/work/rendered/grub.cfg" /EFI/BOOT/grub.cfg \
  -map "$BUILD/work/rendered/isolinux.cfg" /isolinux/isolinux.cfg \
  -map "$BUILD/work/rendered/efiboot.img" /images/efiboot.img \
  -map "$BUILD/work/product.img" /images/product.img \
  -map "$BUILD/work/rendered/initrd.img" /images/pxeboot/initrd.img \
  -map "$BUILD/work/rendered/vmalpha.ks" /vmalpha/vmalpha.ks \
  -map "$PAYLOAD" /vmalpha/features \
  -map "$SOURCE/README.md" /VMALPHA-README.md \
  -map "$SOURCE/ACCEPTANCE.md" /ACCEPTANCE.md \
  -map "$SOURCE/docs" /docs \
  -volid "VMALPHA-${VERSION//./-}" -boot_image any replay \
  -append_partition 2 0xef "$BUILD/work/rendered/efiboot.img" \
  -boot_image any appended_part_as=gpt -commit -end
implantisomd5 "$OUT"
checkisomd5 "$OUT"
# Verify the payload actually embedded in the ISO, including signed hashes.
# In particular, a build-directory symlink must never become an ISO symlink.
EMBEDDED=$(mktemp -d "$BUILD/work/embedded-payload.XXXXXX")
xorriso -osirrox on -indev "$OUT" -extract /vmalpha/features "$EMBEDDED/features" -end
test -d "$EMBEDDED/features" && test ! -L "$EMBEDDED/features"
python3 "$SOURCE/features/verify-payload.py" "$EMBEDDED/features"
sha256sum "$OUT" > "$OUT.sha256"
xorriso -indev "$OUT" -report_el_torito plain > "$BUILD/logs/boot-report.txt" 2>&1
echo "Candidate ISO ready: $OUT"
