import axios from 'axios';
import type {
  LoginResponse,
  PaginatedResponse,
  VirtualMachine,
  Host,
  Cluster,
  StorageClass,
  StorageBackend,
  Volume,
  Network,
  FirewallRule,
  User,
  Role,
  Tenant,
  AIRecommendation,
  AutomationPolicy,
  AIInsights,
  AlertRule,
  AuditLog,
  DashboardMetrics,
  VMSnapshot,
  VMTemplate,
} from '@/types';

const api = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' },
});

// Inject auth token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('nova_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle 401
api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('nova_token');
      localStorage.removeItem('nova_user');
      window.location.href = '/login';
    }
    return Promise.reject(err);
  }
);

// ── Auth ──
export const authAPI = {
  login: (username: string, password: string) =>
    api.post<LoginResponse>('/auth/login', { username, password }),
  getProfile: () => api.get<User>('/auth/profile'),

};

// ── Virtual Machines ──
export const vmAPI = {
  list: (params?: Record<string, string | number>) =>
    api.get<PaginatedResponse<VirtualMachine>>('/vms', { params }),
  get: (id: string) => api.get<VirtualMachine>(`/vms/${id}`),
  create: (data: Partial<VirtualMachine>) => api.post<VirtualMachine>('/vms', data),
  update: (id: string, data: Partial<VirtualMachine>) => api.put<VirtualMachine>(`/vms/${id}`, data),
  delete: (id: string) => api.delete(`/vms/${id}`),
  action: (id: string, action: string, data?: Record<string, unknown>) =>
    api.post(`/vms/${id}/actions/${action}`, data),
  createConsole: (id: string) => api.post<{ endpoint: string; expires_in: number }>(`/vms/${id}/console`),
};

export const templateAPI = {
  list: () => api.get<{ data: VMTemplate[]; items: VMTemplate[]; total: number }>('/storage/templates'),
};

// ── Hosts & Clusters ──
export const hostAPI = {
  list: (params?: Record<string, string | number>) =>
    api.get<PaginatedResponse<Host>>('/hosts', { params }),
  get: (id: string) => api.get<Host>(`/hosts/${id}`),
  maintenance: (id: string, enable: boolean) =>
    api.post(`/hosts/${id}/maintenance`, { enable }),
};

export const clusterAPI = {
  list: () => api.get<Cluster[]>('/clusters'),
  get: (id: string) => api.get<Cluster>(`/clusters/${id}`),
  create: (data: Partial<Cluster>) => api.post<Cluster>('/clusters', data),
};

// ── Storage ──
export const storageAPI = {
  listClassesWithBackends: () =>
    api.get<{ data: StorageClass[]; backends: StorageBackend[]; items: StorageClass[] }>('/storage/classes'),
  addLocalStorage: (data: {
    name: string;
    device?: string;
    mount_path: string;
    fs_type?: string;
    format?: boolean;
    storage_class?: string;
  }) => api.post('/storage/local', data),
  addNFSStorage: (data: {
    name: string;
    server: string;
    export_path: string;
    mount_path?: string;
    mount_options?: string;
    storage_class?: string;
  }) => api.post('/storage/nfs', data),
  addCephStorage: (data: {
    name: string;
    monitors?: string;
    pool: string;
    user?: string;
    key?: string;
    cluster_id?: string;
    storage_class?: string;
    use_cluster?: boolean;
  }) => api.post('/storage/ceph', data),
  backendStatus: (id: string) => api.get(`/storage/backends/${id}/status`),
  mountBackend: (id: string) => api.post(`/storage/backends/${id}/mount`),
  unmountBackend: (id: string) => api.post(`/storage/backends/${id}/unmount`),
  deleteBackend: (id: string, confirm = true) =>
    api.delete(`/storage/backends/${id}`, { params: { confirm: confirm ? 'true' : 'false' } }),
  listClasses: () => api.get<StorageClass[]>('/storage/classes'),
  createClass: (data: Partial<StorageClass>) => api.post<StorageClass>('/storage/classes', data),
  updateClass: (id: string, data: Partial<StorageClass>) => api.put<StorageClass>(`/storage/classes/${id}`, data),

  listVolumes: (params?: Record<string, string | number>) =>
    api.get<PaginatedResponse<Volume>>('/storage/volumes', { params }),
  createVolume: (data: Partial<Volume>) => api.post<Volume>('/storage/volumes', data),
  expandVolume: (id: string, newSizeGB: number) =>
    api.post(`/storage/volumes/${id}/expand`, { new_size_gb: newSizeGB }),
  deleteVolume: (id: string) => api.delete(`/storage/volumes/${id}`),

  listSnapshots: (params?: Record<string, string>) =>
    api.get<VMSnapshot[]>('/storage/snapshots', { params }),
  restoreSnapshot: (id: string) => api.post(`/storage/snapshots/${id}/restore`),
  deleteSnapshot: (id: string) => api.delete(`/storage/snapshots/${id}`),

  listBackups: () => api.get('/storage/backups'),
  createBackup: (data: Record<string, unknown>) => api.post('/storage/backups', data),

  cephHealth: () => api.get('/storage/ceph/health'),
  cephMetrics: (range: string = '1h') => api.get('/storage/ceph/metrics', { params: { range } }),
};

// ── Networking ──
export const networkAPI = {
  list: (params?: Record<string, string | number>) =>
    api.get<PaginatedResponse<Network>>('/networks', { params }),
  get: (id: string) => api.get<Network>(`/networks/${id}`),
  create: (data: Partial<Network>) => api.post<Network>('/networks', data),
  update: (id: string, data: Partial<Network>) => api.put<Network>(`/networks/${id}`, data),
  delete: (id: string) => api.delete(`/networks/${id}`),

  listFirewallRules: (params?: Record<string, string>) =>
    api.get<FirewallRule[]>('/firewalls', { params }),
  createFirewallRule: (data: Partial<FirewallRule>) => api.post<FirewallRule>('/firewalls', data),
  deleteFirewallRule: (id: string) => api.delete(`/firewalls/${id}`),
};

// ── Users / Roles / Tenants ──
export const userAPI = {
  list: () => api.get<User[]>('/users'),
  create: (data: Record<string, unknown>) => api.post<User>('/users', data),
  listRoles: () => api.get<Role[]>('/roles'),
  createRole: (data: Partial<Role>) => api.post<Role>('/roles', data),
  listTenants: () => api.get<Tenant[]>('/tenants'),
  createTenant: (data: Partial<Tenant>) => api.post<Tenant>('/tenants', data),
  listAuditLogs: (params?: Record<string, string | number>) =>
    api.get<PaginatedResponse<AuditLog>>('/audit', { params }),
};

// ── AI / NovaMind ──
export const aiAPI = {
  listRecommendations: (params?: Record<string, string | number>) =>
    api.get<PaginatedResponse<AIRecommendation>>('/ai/recommendations', { params }),
  applyRecommendation: (id: string) => api.post(`/ai/recommendations/${id}/apply`),
  dismissRecommendation: (id: string) => api.post(`/ai/recommendations/${id}/dismiss`),
  getInsights: () => api.get<AIInsights>('/ai/insights'),
  chat: (message: string, context?: string) =>
    api.post('/ai/chat', { message, context }),
  assessMigration: (sourceType: string, vmNames: string[]) =>
    api.post('/ai/migration/assess', { source_type: sourceType, vm_names: vmNames }),

  listPolicies: () => api.get<AutomationPolicy[]>('/ai/policies'),
  createPolicy: (data: Partial<AutomationPolicy>) => api.post<AutomationPolicy>('/ai/policies', data),
  updatePolicy: (id: string, data: Partial<AutomationPolicy>) =>
    api.put<AutomationPolicy>(`/ai/policies/${id}`, data),
};

// ── Monitoring ──
export const monitoringAPI = {
  getDashboard: () => api.get<DashboardMetrics>('/monitoring/dashboard'),
  getEvents: (params?: Record<string, string | number>) =>
    api.get<PaginatedResponse<AuditLog>>('/monitoring/events', { params }),
  listAlertRules: () => api.get<AlertRule[]>('/monitoring/alerts'),
  createAlertRule: (data: Partial<AlertRule>) => api.post<AlertRule>('/monitoring/alerts', data),
  updateAlertRule: (id: string, data: Partial<AlertRule>) =>
    api.put<AlertRule>(`/monitoring/alerts/${id}`, data),
  deleteAlertRule: (id: string) => api.delete(`/monitoring/alerts/${id}`),
};

export default api;

export interface NativeInventory {
  hostname: string; version: string; kernel: string; cpu: number; cpuUsage: number;
  memory: {total: number; used: number}; kvm: boolean; maintenance: boolean;
  vms: {name: string; uuid: string; state: string; cpu: number; memory: number}[];
  pools: {name: string; type: string; path: string; available: string; state: string}[];
  networks: {name: string; mode: string; bridge: string; active: string}[];
}
export interface NativeTask {id: string; host: string; operation: string; status: string; created_at: string}
export interface NativeMetrics {available:boolean;collectorState?:string;unit:string;timestamp:number;message:string;series:{labels:Record<string,string>;values:[number,number|null][]}[]}
export const nativeAPI = {
 files: (host:string,path='')=>api.get<{name:string;path:string;size:number;directory:boolean}[]>(`/native/hosts/${encodeURIComponent(host)}/files`,{params:{path},timeout:30000}),
 metrics: (host:string,metric:string,range:string,scope='host',name='')=>api.get<NativeMetrics>(`/native/hosts/${encodeURIComponent(host)}/metrics`,{params:{scope,metric,range,name},timeout:30000}),
  hosts: () => api.get<{hosts:string[]}>('/native/hosts'),
  inventory: (host: string) => api.get<NativeInventory>(`/native/hosts/${encodeURIComponent(host)}/inventory`),
  operate: (host: string, op: string, args: Record<string, unknown>, key: string) => api.post<{task:NativeTask;replayed?:boolean}>(`/native/hosts/${encodeURIComponent(host)}/operations`, {op,args}, {headers:{'Idempotency-Key':key},timeout:100000}),
  task: (id: string) => api.get<NativeTask>(`/native/tasks/${encodeURIComponent(id)}`),
};
