import argparse,importlib.util,pathlib,unittest
spec=importlib.util.spec_from_file_location('prepare_node',pathlib.Path(__file__).resolve().parents[1]/'features/prepare-node.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class PreparationInputTests(unittest.TestCase):
 def args(self,**changes):
  values=dict(name='node02',api_name='api.example.test',node_ip='192.168.68.90',api_ip='192.168.68.66',node_cidr='192.168.68.0/22',pod_cidr='10.244.0.0/16',service_cidr='10.96.0.0/12',role='control-plane',storage=True);values.update(changes);return argparse.Namespace(**values)
 def test_nonoverlapping_networks(self):self.assertEqual(module.configuration(self.args())['role'],'control-plane')
 def test_overlapping_cidrs_rejected(self):
  with self.assertRaises(ValueError):module.configuration(self.args(pod_cidr='192.168.68.0/24'))
 def test_unbounded_firewall_source_rejected(self):
  with self.assertRaises(ValueError):module.configuration(self.args(node_cidr='0.0.0.0/0'))
 def test_endpoint_outside_selected_network_rejected(self):
  with self.assertRaises(ValueError):module.configuration(self.args(api_ip='203.0.113.1'))
 def test_malformed_names_rejected(self):
  for name in ('node\ninjected','-node','node;id','node..test'):
   with self.assertRaises(ValueError):module.configuration(self.args(name=name))
if __name__=='__main__':unittest.main()
