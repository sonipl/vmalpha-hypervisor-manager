# Native host enrollment

Manager connects to an enrolled host only through the fixed JSON broker command
`sudo -n /usr/libexec/vmalpha-api`. The account cannot receive an interactive
shell, port forwarding, or arbitrary command execution through this identity.

Use [the enrollment script](../appliance/scripts/enroll-native-hosts.sh) on the
Manager appliance as root. It requires a host file with an explicit name,
`host:port`, and a path to the host's public key obtained through an already
trusted console or SSH session:

```text
# name   address              verified host public-key file
kvm11    192.168.71.81:22     /secure/pins/kvm11.ed25519.pub
kvm12    192.168.71.82:22     /secure/pins/kvm12.ed25519.pub
kvm13    192.168.71.83:22     /secure/pins/kvm13.ed25519.pub
```

Run the input check first:

```bash
manager/appliance/scripts/enroll-native-hosts.sh --hosts-file /secure/native-hosts.txt --dry-run
```

Install the public-key line printed by the script on each target through that
trusted management path, then run the script without `--dry-run` and restart
`vmalpha-manager`. The script creates a private Manager identity only when one
does not already exist; it never overwrites that identity. It writes a
service-owned `0600` key and a `0600` `known_hosts` file containing only the
operator-supplied pins. Its configuration update is atomic.

The appliance's first-boot configuration is JSON-compatible YAML. The script
uses the standard library JSON parser deliberately, so it refuses a config
that uses YAML-only syntax instead of risking a partial rewrite. Do not store
host passwords, private keys, or the enrollment host file in this repository or
a reusable appliance image.

Validate after restart as a Platform Admin:

```text
GET /api/v1/native/hosts
GET /api/v1/native/hosts/kvm11/inventory
```

A successful inventory request must show the live hostname and current host
facts. A listed name alone does not prove Manager-to-host access.
