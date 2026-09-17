from pathlib import Path
p=Path('/var/tmp/vmalpha-libvirt-build/SPECS/libvirt.spec');s=p.read_text()
assert '\n%autopatch\n' in s
s=s.replace('\n%autopatch\n', '''
%autopatch
# VM Alpha compatibility patch: Ceph 20.2.4 removed auth_supported.
# Keep authentication required using the supported auth_client_required option.
python3 - <<'PATCHPY'
from pathlib import Path
p=Path('src/storage/storage_backend_rbd.c')
s=p.read_text()
assert s.count('"auth_supported"') == 2
p.write_text(s.replace('"auth_supported"', '"auth_client_required"'))
PATCHPY
''')
lines=s.splitlines()
for i,line in enumerate(lines):
 if line.startswith('Release:'):
  assert '.vmalpha' not in line
  lines[i]=line+'.vmalpha1'
p.write_text('\n'.join(lines)+'\n')
print('Prepared source patch for Ceph authentication option; no authentication bypass')
