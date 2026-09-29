import { useQueries } from '@tanstack/react-query';
import { nativeAPI } from '@/services/api';

type Scope = 'host' | 'vm';
const metrics = [
  ['cpu', 'CPU'], ['memory', 'Memory'], ['network_rx', 'Network receive'], ['network_tx', 'Network transmit'],
  ['disk_read_iops', 'Read IOPS'], ['disk_write_iops', 'Write IOPS'], ['disk_read', 'Read throughput'], ['disk_write', 'Write throughput'],
  ['disk_read_latency', 'Read latency'], ['disk_write_latency', 'Write latency'],
] as const;

function latest(result: any) {
  const values = result?.series?.flatMap((series: any) => series.values || []) || [];
  return values.filter((sample: any) => sample[1] !== null).at(-1)?.[1] as number | undefined;
}
function format(value: number | undefined, unit: string) {
  if (value === undefined) return 'Unavailable';
  if (unit === 'bytes' || unit === 'bytes/s' || unit.startsWith('bytes ')) {
    const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']; let index = 0; let current = value;
    while (Math.abs(current) >= 1024 && index < units.length - 1) { current /= 1024; index++; }
    return `${current.toFixed(current >= 100 ? 0 : 1)} ${units[index]}${unit === 'bytes/s' ? '/s' : ''}`;
  }
  if (unit === 'seconds') return `${(value * 1000).toFixed(value * 1000 >= 10 ? 1 : 2)} ms`;
  return `${value.toFixed(value >= 100 ? 0 : 2)} ${unit}`;
}

export default function MetricsMatrix({ host, scope, vm }: { host: string; scope: Scope; vm?: string }) {
  const queries = useQueries({ queries: metrics.map(([metric]) => ({
    queryKey: ['native-matrix', host, scope, vm || '', metric],
    queryFn: () => nativeAPI.metrics(host, metric, '1h', scope, vm || '').then((response) => response.data),
    enabled: !!host && (scope !== 'vm' || !!vm), retry: false, refetchInterval: 30000,
  })) });
  return <section className="card p-5"><h2 className="font-semibold">Live {scope === 'vm' ? 'VM' : 'host'} metrics</h2>
    <p className="mt-1 text-sm text-gray-500">Prometheus samples refresh every 30 seconds. Values are not estimated.</p>
    <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {metrics.map(([metric, label], index) => {
        const query = queries[index]; const result = query.isError ? undefined : query.data;
        return <div className="rounded border border-gray-200 p-3" key={metric}><p className="text-xs text-gray-500">{label}</p>
          <p className="mt-1 text-lg font-semibold">{query.isPending ? 'Loading…' : format(latest(result), result?.unit || '')}</p>
          <p className="text-xs text-gray-500">{result?.collectorState === 'healthy' ? 'Collector healthy' : result ? 'No current sample' : 'Collector unavailable'}</p>
        </div>;
      })}
    </div>
  </section>;
}
