package handlers

import "strings"

// injectGuestAgent supplies a deterministic cloud-init profile for new Linux
// guests when the requester did not provide one. Existing guest configuration
// is never modified: it may contain ordering or repository requirements that
// Manager cannot safely merge.
func injectGuestAgent(osName, cloudInit string) string {
	if strings.TrimSpace(cloudInit) != "" || !isLinux(osName) {
		return cloudInit
	}
	return `#cloud-config
package_update: false
packages:
  - qemu-guest-agent
  - python3
write_files:
  - path: /usr/local/libexec/vmalpha-guest-metrics
    owner: root:root
    permissions: '0755'
    content: |
      #!/usr/bin/python3
      import json
      from pathlib import Path
      def read(p):
        try: return Path(p).read_text().splitlines()
        except OSError: return []
      mem = {k: int(v.strip().split()[0])*1024 for k,v in (x.split(':',1) for x in read('/proc/meminfo') if ':' in x)}
      cpu = next((x.split() for x in read('/proc/stat') if x.startswith('cpu ')), [])
      interfaces = {}
      for x in read('/proc/net/dev')[2:]:
        name, values = x.split(':',1); values = values.split()
        if name.strip() != 'lo' and len(values) >= 16: interfaces[name.strip()] = {'rx_bytes':int(values[0]),'tx_bytes':int(values[8])}
      disks = {}
      for x in read('/proc/diskstats'):
        row = x.split()
        if len(row) >= 14 and not row[2].startswith(('loop','ram')): disks[row[2]] = {'read_requests':int(row[3]),'read_sectors':int(row[5]),'read_ms':int(row[6]),'write_requests':int(row[7]),'write_sectors':int(row[9]),'write_ms':int(row[10])}
      print(json.dumps({'version':1,'cpu_seconds_total':sum(map(int,cpu[1:]))/100 if cpu else 0,'memory':{'total_bytes':mem.get('MemTotal',0),'available_bytes':mem.get('MemAvailable',0)},'interfaces':interfaces,'disks':disks}))
runcmd:
  - [ systemctl, enable, --now, qemu-guest-agent.service ]
`
}

func isLinux(osName string) bool {
	osName = strings.ToLower(osName)
	return strings.Contains(osName, "linux") || strings.Contains(osName, "rocky") || strings.Contains(osName, "rhel") || strings.Contains(osName, "ubuntu") || strings.Contains(osName, "debian")
}
