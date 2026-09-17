import base64,copy,importlib.util,pathlib,subprocess,tempfile,unittest
spec=importlib.util.spec_from_file_location('serving',pathlib.Path(__file__).resolve().parents[1]/'features/approve-serving.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class ServingIdentityTests(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);subprocess.run(['openssl','req','-new','-newkey','rsa:2048','-nodes','-subj','/O=system:nodes/CN=system:node:node02','-addext','subjectAltName=DNS:node02,IP:192.168.68.90','-keyout',str(root/'key'),'--out',str(root/'request')],check=True,capture_output=True)
   cls.request=base64.b64encode((root/'request').read_bytes()).decode()
 def csr(self):return {'spec':{'request':self.request,'signerName':'kubernetes.io/kubelet-serving','username':'system:node:node02','groups':['system:nodes','system:authenticated'],'usages':['digital signature','key encipherment','server auth']}}
 def test_exact_identity(self):self.assertTrue(module.matches(self.csr(),'node02','192.168.68.90'))
 def test_wrong_san_rejected(self):
  with self.assertRaises(ValueError):module.matches(self.csr(),'node02','192.168.68.91')
 def test_other_node_ignored(self):self.assertFalse(module.matches(self.csr(),'node03','192.168.68.90'))
 def test_client_auth_rejected(self):
  csr=self.csr();csr['spec']['usages'].append('client auth')
  with self.assertRaises(ValueError):module.matches(csr,'node02','192.168.68.90')
 def test_privileged_group_rejected(self):
  csr=self.csr();csr['spec']['groups'].append('system:masters')
  with self.assertRaises(ValueError):module.matches(csr,'node02','192.168.68.90')
 def test_approval_preserves_version_and_does_not_mutate_checked_object(self):
  csr=self.csr();csr['metadata']={'name':'test-csr','uid':'checked-uid','resourceVersion':'42'}
  result=module.approval_document(csr)
  self.assertEqual(result['metadata'],csr['metadata']);self.assertNotIn('status',csr)
  self.assertEqual(result['status']['conditions'][0]['type'],'Approved')
 def test_unversioned_approval_is_rejected(self):
  with self.assertRaises(ValueError):module.approval_document(self.csr())
if __name__=='__main__':unittest.main()
