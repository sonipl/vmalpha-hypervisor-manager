#!/usr/bin/python3
"""Restricted SSH command gateway for the Manager service identity.

The enrolled key cannot obtain a shell.  It can only start the fixed JSON
broker or the per-guest VNC byte proxy, whose own authorization checks remain
in effect.
"""
import os
import re

command = os.environ.get("SSH_ORIGINAL_COMMAND", "")
if command == "sudo -n /usr/libexec/vmalpha-api":
    os.execv("/usr/bin/sudo", ["sudo", "-n", "/usr/libexec/vmalpha-api"])

match = re.fullmatch(r"sudo -n /usr/libexec/vmalpha-vnc ([A-Za-z][A-Za-z0-9_.-]{0,62})", command)
if match:
    os.execv("/usr/bin/sudo", ["sudo", "-n", "/usr/libexec/vmalpha-vnc", match.group(1)])

raise SystemExit("Manager SSH identity is restricted to the VM Alpha broker and console proxy")
