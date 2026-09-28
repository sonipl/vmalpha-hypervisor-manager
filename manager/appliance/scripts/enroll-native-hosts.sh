#!/usr/bin/env bash
# Enroll Manager access to native hypervisor brokers using an operator-verified key list.
# Run on the Manager appliance as root. This script never contacts a host or accepts
# a host key from the network; install the emitted restricted public key through an
# existing trusted management path before applying this configuration.
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: enroll-native-hosts.sh --hosts-file FILE [options]

Each non-comment line in FILE is:
  NAME ADDRESS HOST_PUBLIC_KEY_FILE

ADDRESS must be host:port. HOST_PUBLIC_KEY_FILE must contain the public host key
retrieved through an already trusted channel, never ssh-keyscan or TOFU.

Options:
  --config PATH       Manager JSON-compatible YAML configuration
                      (default: /etc/novasphere/config.yaml)
  --identity-dir DIR  Manager enrollment identity directory
                      (default: /etc/novasphere/native-hosts)
  --dry-run           Validate inputs and show the restricted authorized_keys line;
                      do not change files
  -h, --help          Show this help
USAGE
}

config=/etc/novasphere/config.yaml
identity_dir=/etc/novasphere/native-hosts
hosts_file=
dry_run=false
while (($#)); do
  case "$1" in
    --hosts-file) hosts_file=${2:?}; shift 2 ;;
    --config) config=${2:?}; shift 2 ;;
    --identity-dir) identity_dir=${2:?}; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n "$hosts_file" && -r "$hosts_file" ]] || { echo "A readable --hosts-file is required" >&2; exit 2; }
[[ -f "$config" ]] || { echo "Manager config not found: $config" >&2; exit 2; }

entries=()
while read -r name address keyfile extra; do
  [[ -z "${name:-}" || "$name" == \#* ]] && continue
  [[ -z "${address:-}" || -z "${keyfile:-}" || -n "${extra:-}" ]] && { echo "Invalid host entry for ${name:-unknown}" >&2; exit 2; }
  [[ "$name" =~ ^[a-z0-9][a-z0-9.-]{0,61}[a-z0-9]$ ]] || { echo "Invalid host name: $name" >&2; exit 2; }
  [[ "$address" =~ ^[^:[:space:]]+:[0-9]{1,5}$ ]] || { echo "Address must be host:port: $address" >&2; exit 2; }
  [[ -f "$keyfile" && ! -L "$keyfile" ]] || { echo "Pinned public key is not a regular file: $keyfile" >&2; exit 2; }
  ssh-keygen -lf "$keyfile" >/dev/null || { echo "Invalid SSH public key: $keyfile" >&2; exit 2; }
  entries+=("$name|$address|$keyfile")
done < "$hosts_file"
((${#entries[@]})) || { echo "No host entries supplied" >&2; exit 2; }

if $dry_run; then
  echo "Validated ${#entries[@]} supplied pinned host key(s)."
  echo "Apply only after this public key is installed on every target root account:"
  echo 'restrict,command="/usr/bin/sudo -n /usr/libexec/vmalpha-api" <MANAGER_PUBLIC_KEY> manager-native-telemetry'
  exit 0
fi

getent passwd vmalpha >/dev/null || { echo "Service account vmalpha does not exist" >&2; exit 1; }
install -d -m 0750 -o vmalpha -g vmalpha "$identity_dir"
private_key="$identity_dir/manager_native"
public_key="$private_key.pub"
known_hosts="$identity_dir/known_hosts"
if [[ ! -e "$private_key" ]]; then
  umask 077
  ssh-keygen -q -t rsa -b 3072 -N '' -f "$private_key" -C manager-native-telemetry
  chown vmalpha:vmalpha "$private_key" "$public_key"
elif [[ ! -f "$private_key" || -L "$private_key" ]]; then
  echo "Refusing to replace non-regular identity: $private_key" >&2; exit 1
fi
chmod 0600 "$private_key"
chown vmalpha:vmalpha "$private_key" "$public_key"

# Rebuild only the script-managed pinned-key file. Its content is entirely derived
# from operator-supplied files, and replacement is atomic.
tmp=$(mktemp "$identity_dir/.known_hosts.XXXXXX")
trap 'rm -f "$tmp"' EXIT
for entry in "${entries[@]}"; do
  IFS='|' read -r name address keyfile <<<"$entry"
  host=${address%:*}; port=${address##*:}
  key=$(awk 'NF >= 2 && $1 !~ /^#/ {print $1" "$2; exit}' "$keyfile")
  [[ -n "$key" ]] || { echo "No public key in $keyfile" >&2; exit 1; }
  printf '[%s]:%s %s\n' "$host" "$port" "$key" >> "$tmp"
  # The unbracketed entry supports local ssh diagnostics; the bracketed entry is
  # what golang.org/x/crypto/ssh knownhosts uses for host:port callback addresses.
  printf '%s %s\n' "$host" "$key" >> "$tmp"
done
chmod 0600 "$tmp"
chown vmalpha:vmalpha "$tmp"
if [[ ! -f "$known_hosts" ]] || ! cmp -s "$tmp" "$known_hosts"; then
  install -m 0600 -o vmalpha -g vmalpha "$tmp" "$known_hosts"
fi

python3 - "$config" "$private_key" "$known_hosts" "${entries[@]}" <<'PY'
import json, os, sys, tempfile
config_path, private_key, known_hosts, *entries = sys.argv[1:]
with open(config_path, encoding="utf-8") as stream:
    config = json.load(stream)
hosts = {}
for entry in entries:
    name, address, _ = entry.split("|", 2)
    hosts[name] = {"address": address, "user": "root", "private_key_file": private_key, "known_hosts_file": known_hosts}
config["native_hosts"] = hosts
fd, temp_path = tempfile.mkstemp(prefix=".config.native-hosts.", dir=os.path.dirname(config_path))
try:
    with os.fdopen(fd, "w", encoding="utf-8") as stream:
        json.dump(config, stream, indent=2)
        stream.write("\n")
    os.chown(temp_path, os.stat(config_path).st_uid, os.stat(config_path).st_gid)
    os.chmod(temp_path, 0o600)
    os.replace(temp_path, config_path)
finally:
    if os.path.exists(temp_path): os.unlink(temp_path)
PY

echo "Enrollment files written. Install this exact restricted key on each target through a trusted channel:"
printf 'restrict,command="/usr/bin/sudo -n /usr/libexec/vmalpha-api" '
cat "$public_key"
echo "Restart vmalpha-manager, then validate GET /api/v1/native/hosts/<name>/inventory as a Platform Admin."
