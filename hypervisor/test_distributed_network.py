import copy,importlib.util,pathlib,tempfile,unittest
from unittest.mock import patch
spec=importlib.util.spec_from_file_location('network',pathlib.Path(__file__).with_name('vmalpha_distributed_network.py'));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
ID='12345678-1234-1234-1234-123456789abc'
def request():return {'id':ID,'action':'create','spec':{'name':'VM networks','bridge':'brguest','uplink':'ens224','mtu':1500,'hosts':['testhost'],'port_groups':[{'name':'untagged','vlan_id':0},{'name':'VLAN 42','vlan_id':42}]}}
def initial():return {'interfaces':[{'name':'ens224','physical':True,'master':None,'mtu':1500,'addresses':[],'routed':False,'vm_referenced':False}],'connections':'','profile_interfaces':{},'vm_references':[]}
class NetworkTests(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.state=patch.object(m,'STATE',pathlib.Path(self.tmp.name));self.state.start();self.addCleanup(self.state.stop)
  for name in ('gethostname','getfqdn'):
   p=patch.object(m.socket,name,return_value='testhost');p.start();self.addCleanup(p.stop)
  self.calls=[]
 def command(self,*args,**kw):self.calls.append(args);return ''
 def test_validation_and_mapping(self):
  r=request();m.validate(r);rows=m.profiles(ID,r['spec']);self.assertEqual(len(rows),4);self.assertTrue(all(len(p['ifname'])<=15 for p in rows))
  for field,value in [('bridge','br;id'),('mtu',1279),('hosts',['different'])]:
   bad=copy.deepcopy(r);bad['spec'][field]=value
   with self.assertRaises(ValueError):m.validate(bad)
  r['spec']['port_groups'][1]['vlan_id']=4095
  with self.assertRaises(ValueError):m.validate(r)
 def test_preview_readonly_and_stale_apply(self):
  with patch.object(m,'snapshot',return_value=initial()):
   result=m.handle('network.distributed.preview',request(),self.command);self.assertTrue(result['supported']);self.assertEqual(self.calls,[])
   r=request();r['expected_fingerprint']='stale'
   with self.assertRaises(ValueError):m.handle('network.distributed.apply',r,self.command)
   self.assertEqual(self.calls,[]);self.assertEqual(list(m.STATE.iterdir()),[])
 def test_management_and_inactive_vm_protected(self):
  for field,value in [('addresses',[{'local':'192.168.71.81'}]),('routed',True),('vm_referenced',True),('master','brmanagement')]:
   data=initial();data['interfaces'][0][field]=value
   with patch.object(m,'snapshot',return_value=data),self.assertRaises(ValueError):m.preflight(request(),self.command)
  self.assertEqual(self.calls,[])
 def test_apply_and_delete_restore_mtu(self):
  r=request();before=initial();after=initial();after['interfaces']=[];lines=[]
  for p in m.profiles(ID,r['spec']):
   after['interfaces'].append({'name':p['ifname'],'mtu':1500,'master':p.get('master'),'physical':p['ifname']=='ens224','addresses':[],'routed':False,'vm_referenced':False})
   lines.append('uuid:'+p['id']+':ethernet:'+p['ifname'])
  after['connections']='\n'.join(lines);after['profile_interfaces']={'uuid':''}
  with patch.object(m,'snapshot',return_value=before):r['expected_fingerprint']=m.preflight(r,self.command)['fingerprint']
  with patch.object(m,'snapshot',side_effect=[before,after]):result=m.handle('network.distributed.apply',r,self.command)
  self.assertTrue(result['applied']);self.assertEqual(m.load(ID)['state'],'applied')
  r['action']='delete'
  with patch.object(m,'snapshot',return_value=after):r['expected_fingerprint']=m.preflight(r,self.command)['fingerprint']
  with patch.object(m,'snapshot',side_effect=[after,before]):result=m.handle('network.distributed.apply',r,self.command)
  self.assertTrue(result['applied']);self.assertIsNone(m.load(ID));self.assertIn(('ip','link','set','dev','ens224','mtu','1500'),self.calls)
 def test_failure_removes_only_created_profiles(self):
  r=request()
  with patch.object(m,'snapshot',return_value=initial()):
   r['expected_fingerprint']=m.preflight(r,self.command)['fingerprint']
   def fail(*args,**kw):
    self.calls.append(args)
    if 'up' in args:raise ValueError('simulated activation failure')
    return ''
   result=m.handle('network.distributed.apply',r,fail)
  self.assertFalse(result['applied']);self.assertEqual(result['rollback'],'removed_created_profiles')
  deletes=[a[-1] for a in self.calls if a[:3]==('nmcli','connection','delete')];self.assertEqual(set(deletes),{p['id'] for p in m.profiles(ID,r['spec'])})
if __name__=='__main__':unittest.main()
