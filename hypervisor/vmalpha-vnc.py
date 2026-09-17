#!/usr/bin/python3
"""Root-only, domain-scoped VNC stream proxy; no TCP listener is created."""
import os,pathlib,re,selectors,socket,subprocess,sys,xml.etree.ElementTree as ET
sys.path.insert(0,'/usr/share/vmalpha')
from vmalpha_auth import require
if len(sys.argv)!=2 or not re.fullmatch(r'[A-Za-z][A-Za-z0-9_.-]{0,62}',sys.argv[1]):sys.exit('Invalid guest')
try:require('vm.console',sys.argv[1])
except Exception as e:sys.exit(str(e))
x=ET.fromstring(subprocess.check_output(['virsh','dumpxml',sys.argv[1]]));g=x.find("devices/graphics[@type='vnc']")
if g is None:sys.exit('Guest has no VNC console')
listen=g.find("listen[@type='socket']");p=(listen.get('socket') if listen is not None else None) or g.get('socket')
if p:
 p=pathlib.Path(p).resolve()
 if not any(p.is_relative_to(pathlib.Path(root)) for root in ['/run/libvirt/qemu','/var/lib/libvirt/qemu']):sys.exit('Unexpected console socket')
 s=socket.socket(socket.AF_UNIX);s.connect(str(p))
else:
 port=int(g.get('port','0'))
 if not 5900<=port<=65535 or g.get('listen') not in ('127.0.0.1','::1'):sys.exit('Console must listen only on localhost')
 s=socket.create_connection(('127.0.0.1',port))
sel=selectors.DefaultSelector();sel.register(0,selectors.EVENT_READ);sel.register(s,selectors.EVENT_READ)
try:
 while True:
  require('vm.console',sys.argv[1])
  for key,_ in sel.select(timeout=1):
   b=os.read(0,65536) if key.fileobj==0 else s.recv(65536)
   if not b:sys.exit(0)
   if key.fileobj==0:s.sendall(b)
   else:
    while b:b=b[os.write(1,b):]
finally:s.close()
