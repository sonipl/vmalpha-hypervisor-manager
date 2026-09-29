#!/usr/bin/python3
"""Emit bounded guest telemetry for execution through qemu-guest-agent.

The program never opens a TCP listener and accepts no arguments.  The host
starts it through the already authenticated virtio qemu-guest-agent channel.
"""
import json
from pathlib import Path

def lines(path):
    try: return Path(path).read_text().splitlines()
    except OSError: return []

def meminfo():
    values = {}
    for line in lines('/proc/meminfo'):
        key, value = line.split(':', 1)
        values[key] = int(value.strip().split()[0]) * 1024
    return {'total_bytes': values.get('MemTotal', 0), 'available_bytes': values.get('MemAvailable', 0)}

def cpu_seconds():
    fields = next((line.split() for line in lines('/proc/stat') if line.startswith('cpu ')), [])
    return sum(int(value) for value in fields[1:]) / 100.0 if fields else 0

def interfaces():
    result = {}
    for line in lines('/proc/net/dev')[2:]:
        name, values = line.split(':', 1)
        if name.strip() == 'lo': continue
        counters = values.split()
        if len(counters) >= 16:
            result[name.strip()] = {'rx_bytes': int(counters[0]), 'tx_bytes': int(counters[8])}
    return result

def disks():
    result = {}
    for line in lines('/proc/diskstats'):
        row = line.split()
        if len(row) < 14 or row[2].startswith(('loop', 'ram')): continue
        result[row[2]] = {'read_requests': int(row[3]), 'read_sectors': int(row[5]), 'read_ms': int(row[6]),
                          'write_requests': int(row[7]), 'write_sectors': int(row[9]), 'write_ms': int(row[10])}
    return result

print(json.dumps({'version': 1, 'cpu_seconds_total': cpu_seconds(), 'memory': meminfo(),
                  'interfaces': interfaces(), 'disks': disks()}, separators=(',', ':')))
