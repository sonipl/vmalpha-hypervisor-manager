#!/usr/bin/python3
import os,re,sys,subprocess
sys.path.insert(0,'/usr/share/vmalpha')
from vmalpha_auth import require
if len(sys.argv)!=2 or not re.fullmatch(r'[A-Za-z][A-Za-z0-9_.-]{0,62}',sys.argv[1]):sys.exit('Invalid guest name')
try:require('vm.console',sys.argv[1])
except Exception as e:sys.exit(str(e))
p=subprocess.Popen(['/usr/bin/virsh','console',sys.argv[1],'--safe'])
try:
 while True:
  try:sys.exit(p.wait(timeout=1))
  except subprocess.TimeoutExpired:require('vm.console',sys.argv[1])
except Exception as e:
 p.terminate()
 try:p.wait(timeout=2)
 except subprocess.TimeoutExpired:p.kill()
 sys.exit(str(e))
