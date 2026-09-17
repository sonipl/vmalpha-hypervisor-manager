# Feature lifecycle helpers — development status

These helpers are staged for release integration. Input/configuration tests have
passed; complete clean-media initialization, same-cluster joins and expansion
acceptance are still pending. They must not be presented as a fully tested
operator workflow until those live checks pass.

* `verify-payload.py` validates the signed bundled manifest and every listed file.
* `install-offline.sh` installs local feature packages with package signatures and
  the selected native RBD compatibility driver, then enables image loading.
* `load-images.sh` imports verified archives into containerd/Podman. Archives stay
  on the installed OS. Missing references after garbage collection trigger a
  reload rather than requiring an Internet pull.
* `prepare-node.py` validates node/API identity, nonoverlapping management/pod/
  service networks and the selected role. With `--apply`, it requires a local
  address, synchronized clock, enforcing SELinux, active firewall and installed
  runtimes before applying explicit hostname/network prerequisites. It does not
  create cluster membership or select any data disk.
* `init-cluster.py` initializes only a new prepared control plane, with Secrets
  encryption, authenticated kubelet settings and bundled Flannel. Existing etcd
  or cluster identity causes refusal. `--run-workloads` explicitly permits
  workloads on the initial control plane. Bootstrap credentials are short-lived
  and remain in a root-protected log.
* `join-node.py` reads a root-owned protected JoinConfiguration, requires pinned
  CA discovery and exact endpoint/node/role matching, and never resets an
  existing identity. Control-plane joining requires the existing cluster's
  protected encryption-provider configuration as well as certificate material.
* `approve-serving.py` checks the selected registered node address, requesting
  identity/groups, signed CSR subject/SANs and server-only usage before optional
  approval. The update retains the checked resource version, so a replacement
  race fails. Do not approve arbitrary pending CSRs in bulk.
* `init-storage.py` initializes monitor/manager roles from the bundled Ceph
  image only on a prepared storage node with no existing Ceph state. It never
  consumes an OSD disk. Non-HA single-host settings require `--single-host`.
* `configure-shutdown.py` applies tested kubelet shutdown grace and ordering.
  It assigns critical CSI priorities only when that driver is configured.

Run each helper with `--help` for required inputs. The preparation and bootstrap
helpers default to validation; applying changes requires their explicit flag.
Do not put join tokens, certificate transfer keys, host keys, Ceph credentials,
etcd backups or runtime state into this source tree or into a reusable image.

A successful configuration parse is not a successful install, Ready node,
healthy Ceph quorum or persistent workload. Use the release acceptance gates
and the operator/expansion guides under `../docs`.
