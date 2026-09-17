// ── Core Types ──

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

// ── Virtual Machines ──

export type VMStatus = 'Running' | 'Stopped' | 'Paused' | 'Migrating' | 'Provisioning' | 'Error';

export interface VirtualMachine {
  id: string;
  name: string;
  namespace: string;
  status: VMStatus;
  description: string;
  os: string;
  os_version: string;
  vcpus: number;
  memory_mb: number;
  cpu_sockets: number;
  cpu_cores: number;
  cpu_threads: number;
  cpu_model: string;
  host_node: string;
  ip_address: string;
  secure_boot: boolean;
  vtpm: boolean;
  gpu_device: string;
  eviction_strategy: string;
  template_id?: string;
  owner_id: string;
  tenant_id: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  cloud_init: string;
  disks: VMDisk[];
  nics: VMNIC[];
  created_at: string;
  updated_at: string;
}

export interface VMDisk {
  id: string;
  vm_id: string;
  name: string;
  size_gb: number;
  storage_class: string;
  ceph_pool: string;
  bus: string;
  cache_mode: string;
  bootable: boolean;
  hot_plugged: boolean;
}

export interface VMNIC {
  id: string;
  vm_id: string;
  name: string;
  network_id: string;
  mac_address: string;
  model: string;
  ip_address: string;
  type: string;
  hot_plugged: boolean;
}

export interface VMSnapshot {
  id: string;
  vm_id: string;
  name: string;
  description: string;
  size_mb: number;
  status: string;
  created_by: string;
  created_at: string;
}

// ── Templates ──

export interface VMTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  os: string;
  os_version: string;
  vcpus: number;
  memory_mb: number;
  disk_gb: number;
  storage_class: string;
  image_url: string;
  is_public: boolean;
  labels: Record<string, string>;
}

// ── Compute ──

export type HostStatus = 'Ready' | 'Maintenance' | 'Draining' | 'Error' | 'Offline';

export interface Host {
  id: string;
  name: string;
  status: HostStatus;
  ip_address: string;
  cluster_id: string;
  cpu_model: string;
  cpu_cores: number;
  memory_mb: number;
  nic_count: number;
  gpu_devices: Record<string, unknown>[];
  osd_count: number;
  kernel_version: string;
  os_version: string;
  labels: Record<string, string>;
  numa_nodes: number;
}

export interface Cluster {
  id: string;
  name: string;
  description: string;
  cpu_overcommit: number;
  memory_overcommit: number;
  default_storage_class: string;
  hosts: Host[];
}

// ── Storage ──

export interface StorageClass {
  replication_factor?: number;
  status?: string;
  pool?: string;
  id: string;
  name: string;
  description: string;
  provisioner: string;
  ceph_pool: string;
  replica_count: number;
  is_default: boolean;
  max_iops: number;
  max_throughput_mb: number;
  encryption: boolean;
  compression: boolean;
}

export interface Volume {
  id: string;
  name: string;
  namespace: string;
  size_gb: number;
  used_gb: number;
  storage_class: string;
  ceph_pool: string;
  status: string;
  access_mode: string;
  vm_id?: string;
}

// ── Networking ──

export interface Network {
  id: string;
  name: string;
  namespace: string;
  type: string;
  vlan_id: number;
  subnet: string;
  gateway: string;
  dhcp_enabled: boolean;
  mtu: number;
  tenant_id: string;
}

export interface FirewallRule {
  id: string;
  name: string;
  namespace: string;
  direction: 'Ingress' | 'Egress';
  protocol: string;
  port_range: string;
  source: string;
  dest: string;
  action: 'Allow' | 'Deny';
  priority: number;
}

// ── Auth ──

export interface User {
  id: string;
  username: string;
  email: string;
  full_name: string;
  role: string;
  tenant_id: string;
}

export interface Role {
  id: string;
  name: string;
  description: string;
  scope: string;
  is_built_in: boolean;
  permissions: Record<string, unknown>;
}

export interface Tenant {
  id: string;
  name: string;
  description: string;
  max_vcpus: number;
  max_memory_mb: number;
  max_storage_gb: number;
  max_vms: number;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: User;
}

// ── AI ──

export interface AIRecommendation {
  id: string;
  type: string;
  severity: string;
  target_type: string;
  target_id: string;
  target_name: string;
  title: string;
  description: string;
  confidence: number;
  status: string;
  action_json: Record<string, unknown>;
  created_at: string;
}

export interface AutomationPolicy {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  auto_apply: boolean;
  trigger: Record<string, unknown>;
  action: Record<string, unknown>;
  cooldown_min: number;
}

export interface AIInsights {
  recommendations: AIRecommendation[];
  recommendation_count: number;
  capacity_forecast: {
    cpu_days_until_80pct: number;
    memory_days_until_80pct: number;
    storage_days_until_80pct: number;
  };
  anomalies_24h: number;
  health_score: number;
}

// ── Monitoring ──

export interface AlertRule {
  id: string;
  name: string;
  severity: string;
  expression: string;
  duration: string;
  enabled: boolean;
}

export interface AuditLog {
  id: string;
  timestamp: string;
  user_id: string;
  username: string;
  action: string;
  resource: string;
  resource_id: string;
  details: Record<string, unknown>;
  ip_address: string;
  source: string;
  success: boolean;
}

export interface DashboardMetrics {
  vms: {
    total: number;
    running: number;
    stopped: number;
    error: number;
    migrating: number;
    paused: number;
    provisioning: number;
  };
  hosts: {
    total: number;
    ready: number;
  };
  cluster_health: string;
  telemetry_status: string;
  utilization: {
    cpu_percent: number | null;
    memory_percent: number | null;
    storage_percent: number | null;
    network_mbps: number | null;
  };
}

// ── WebSocket ──

export interface WSEvent {
  type: string;
  resource: string;
  id?: string;
  data?: unknown;
  time: string;
}

export type StorageBackend = {
  id: string;
  type: 'local' | 'nfs' | 'ceph';
  name: string;
  storage_class: string;
  status: string;
  message?: string;
  capacity_bytes?: number;
  created_at?: string;
  config?: Record<string, unknown>;
};

