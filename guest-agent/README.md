# VM Alpha Guest Agent

The guest agent supplements the host-side libvirt counters. It uses the
existing authenticated QEMU virtio guest-agent channel; it opens no guest
network port, stores no Manager credential, and cannot accept arbitrary
commands from a network client.

Install it while producing a template:

```bash
sudo ./install.sh
```

The template must have a QEMU guest-agent virtio channel. VM Alpha’s create
workflow will request that channel for agent-enabled templates. The agent
reports guest CPU time, available and total memory, per-interface byte
counters, and per-disk request, sector, and timing counters. If the channel
or package is absent, Manager retains the measured libvirt metrics and marks
guest-only values unavailable.
