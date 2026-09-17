import clsx from 'clsx';

interface StatusBadgeProps {
  status: string;
  size?: 'sm' | 'md';
}

const statusMap: Record<string, { color: string; label: string }> = {
  running:      { color: 'status-running',   label: 'Running' },
  stopped:      { color: 'status-stopped',   label: 'Stopped' },
  paused:       { color: 'status-warning',   label: 'Paused' },
  error:        { color: 'status-error',     label: 'Error' },
  failed:       { color: 'status-error',     label: 'Failed' },
  creating:     { color: 'status-warning',   label: 'Creating' },
  migrating:    { color: 'status-warning',   label: 'Migrating' },
  starting:     { color: 'status-warning',   label: 'Starting' },
  stopping:     { color: 'status-warning',   label: 'Stopping' },
  maintenance:  { color: 'status-warning',   label: 'Maintenance' },
  healthy:      { color: 'status-running',   label: 'Healthy' },
  ready:        { color: 'status-running',   label: 'Ready' },
  draining:     { color: 'status-warning',   label: 'Draining' },
  offline:      { color: 'status-stopped',   label: 'Offline' },
  degraded:     { color: 'status-warning',   label: 'Degraded' },
  active:       { color: 'status-running',   label: 'Active' },
  inactive:     { color: 'status-stopped',   label: 'Inactive' },
  available:    { color: 'status-running',   label: 'Available' },
  bound:        { color: 'status-running',   label: 'Bound' },
  pending:      { color: 'status-warning',   label: 'Pending' },
  applied:      { color: 'status-running',   label: 'Applied' },
  dismissed:    { color: 'status-stopped',   label: 'Dismissed' },
  critical:     { color: 'status-error',     label: 'Critical' },
  warning:      { color: 'status-warning',   label: 'Warning' },
  info:         { color: 'status-running',   label: 'Info' },
};

export default function StatusBadge({ status, size = 'sm' }: StatusBadgeProps) {
  const s = statusMap[status?.toLowerCase()] ?? { color: 'status-stopped', label: status };

  return (
    <span
      className={clsx(
        'status-badge',
        s.color,
        size === 'md' && 'text-xs px-3 py-1',
      )}
    >
      {s.label}
    </span>
  );
}
