#!/usr/bin/python3
"""Destructive tests are confined to alpha020-* disposable resources on the dev VM."""
import json,subprocess,secrets
API='/usr/libexec/vmalpha-api'
def call(op,args={},ok=True):
 p=subprocess.run([API],input=json.dumps(dict(op=op,args=args)),text=True,capture_output=True)
 r=json.loads(p.stdout)
 assert r['ok']==ok, (op,r.get('error'))
 print(('PASS ' if ok else 'PASS rejected ')+op,flush=True)
 return r.get('result')
call('inventory');call('security');call('pool-create',{'name':'../../outside'},False)
call('network-create',{'name':'alpha020-overlap','mode':'nat','subnet':'192.168.68.0/24'},False)
call('vm-action',{'name':'x; touch /tmp/no','action':'start'},False)
call('files',{'path':'/etc'},False)
call('service',{'name':'firewalld','action':'stop'},False)
call('firewall',{'service':'cockpit','enabled':False},False)
call('user-lock',{'name':'admin','locked':True},False)
call('pool-create',{'name':'alpha020-storage'})
call('volume-create',{'pool':'alpha020-storage','name':'alpha020-disk','size':1})
call('network-create',{'name':'alpha020-isolated','mode':'isolated','subnet':'10.77.21.0/24'})
call('network-create',{'name':'alpha020-nat','mode':'nat','subnet':'10.77.22.0/24'})
call('user-create',{'name':'alpha020-reader','password':secrets.token_urlsafe(24),'role':'OS user'})
p=subprocess.run(['runuser','-u','alpha020-reader','--','sudo','-n',API],input='{"op":"inventory"}',text=True,capture_output=True)
assert p.returncode!=0
print('PASS non-administrator server-side denial',flush=True)
call('user-lock',{'name':'alpha020-reader','locked':True})
call('user-lock',{'name':'alpha020-reader','locked':False})
call('lockout',{'deny':5,'unlock':900})
call('banner',{'text':'VM Alpha Hypervisor 1.0\nAuthorized system access only.'})
call('firewall',{'service':'http','enabled':True})
call('firewall',{'service':'http','enabled':False})
call('service',{'name':'chronyd','action':'restart'})
call('maintenance',{'enabled':True},False)
print('PASS runtime acceptance subset; no host power or existing guest changes',flush=True)
