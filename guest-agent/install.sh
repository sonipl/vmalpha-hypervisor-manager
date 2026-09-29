#!/bin/bash
# VM Alpha Guest Agent: install in Rocky/RHEL compatible templates or guests.
set -euo pipefail
dnf -y install qemu-guest-agent python3
install -D -o root -g root -m 0755 "$(dirname "$0")/vmalpha-guest-metrics.py" /usr/local/libexec/vmalpha-guest-metrics
systemctl enable --now qemu-guest-agent.service
