import copy,importlib.util,pathlib,unittest
spec=importlib.util.spec_from_file_location('join_node',pathlib.Path(__file__).resolve().parents[1]/'features/join-node.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class JoinBoundaryTests(unittest.TestCase):
 def setUp(self):
  self.node={'name':'node02','api_name':'api.example.test','node_ip':'192.168.68.90','role':'worker'}
  self.data={'apiVersion':'kubeadm.k8s.io/v1beta4','kind':'JoinConfiguration','discovery':{'bootstrapToken':{'token':'abcdef.0123456789abcdef','apiServerEndpoint':'api.example.test:6443','caCertHashes':['sha256:'+'a'*64]}},'nodeRegistration':{'name':'node02','criSocket':'unix:///run/containerd/containerd.sock'}}
 def test_worker_configuration(self):self.assertEqual(module.validated(self.data,self.node),self.data)
 def test_cannot_skip_ca_verification(self):
  self.data['discovery']['bootstrapToken']['unsafeSkipCAVerification']=True
  with self.assertRaises(ValueError):module.validated(self.data,self.node)
 def test_missing_ca_pin_rejected(self):
  self.data['discovery']['bootstrapToken']['caCertHashes']=[]
  with self.assertRaises(ValueError):module.validated(self.data,self.node)
 def test_wrong_endpoint_rejected(self):
  self.data['discovery']['bootstrapToken']['apiServerEndpoint']='other.example.test:6443'
  with self.assertRaises(ValueError):module.validated(self.data,self.node)
 def test_wrong_node_rejected(self):
  self.data['nodeRegistration']['name']='node01'
  with self.assertRaises(ValueError):module.validated(self.data,self.node)
 def test_worker_cannot_elevate_to_control_plane(self):
  self.data['controlPlane']={}
  with self.assertRaises(ValueError):module.validated(self.data,self.node)
 def test_control_plane_requires_own_address(self):
  self.node['role']='control-plane';self.data['controlPlane']={'certificateKey':'b'*64,'localAPIEndpoint':{'advertiseAddress':'192.168.68.66','bindPort':6443}}
  with self.assertRaises(ValueError):module.validated(self.data,self.node)
 def test_control_plane_validated(self):
  self.node['role']='control-plane';self.data['controlPlane']={'certificateKey':'b'*64,'localAPIEndpoint':{'advertiseAddress':self.node['node_ip'],'bindPort':6443}}
  self.assertEqual(module.validated(self.data,self.node),self.data)
if __name__=='__main__':unittest.main()
