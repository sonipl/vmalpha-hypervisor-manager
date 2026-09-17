import clsx from 'clsx';

interface UtilizationGaugeProps {
  label: string;
  value: number;
  max?: number;
  unit?: string;
  size?: 'sm' | 'md';
}

export default function UtilizationGauge({
  label, value, max = 100, unit = '%', size = 'md',
}: UtilizationGaugeProps) {
  const pct = Math.min((value / max) * 100, 100);
  const color =
    pct >= 90 ? 'bg-red-500' :
    pct >= 75 ? 'bg-amber-500' :
    'bg-nova-500';

  return (
    <div className={clsx(size === 'sm' && 'text-xs')}>
      <div className="flex items-center justify-between mb-1.5">
        <span className="text-gray-600 dark:text-gray-400 font-medium">{label}</span>
        <span className="font-mono font-semibold text-gray-900 dark:text-white tabular-nums">
          {value}{unit}
        </span>
      </div>
      <div className="h-2 bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
        <div
          className={clsx('h-full rounded-full transition-all duration-500', color)}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
