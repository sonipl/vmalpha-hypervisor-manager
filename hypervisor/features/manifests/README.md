# Bundled cluster manifests

`kube-flannel-0.28.9.yaml` is the upstream versioned Flannel deployment used by
the integration cluster; its pod network defaults to 10.244.0.0/16. The product
initializer must render the selected pod CIDR before applying it.

`ceph-csi-operator-1.0.5.yaml` is the declarative operator/CRD/RBAC manifest used
by the integration fixture, generated from the retained upstream 1.0.5 source.
It contains no live Secret objects or resource identities. Its operator image
is bundled alongside the tested CSI driver and sidecars. The upstream license
is retained here. Cluster-specific Ceph connections, restricted credentials and
StorageClasses must be created during authenticated setup, never baked into
these manifests.

These files are staged for the next source build. Their presence does not mean
that the general initializer, a final ISO, or expanded-cluster acceptance has
passed. Do not download a floating latest manifest at feature activation time.
