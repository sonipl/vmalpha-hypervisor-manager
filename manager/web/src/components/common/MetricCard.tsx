import React from 'react';
import { TrendingUp, TrendingDown, Minus } from 'lucide-react';
import clsx from 'clsx';

interface MetricCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon?: React.ElementType;
  trend?: number;
  trendLabel?: string;
  color?: 'nova' | 'copper' | 'green' | 'red' | 'blue' | 'amber' | 'neutral';
  statusTone?: 'green' | 'amber' | 'red' | 'neutral';
}

const statusText = {green: 'text-emerald-700 dark:text-emerald-400', amber: 'text-amber-700 dark:text-amber-400', red: 'text-red-700 dark:text-red-400', neutral: 'text-gray-600 dark:text-gray-400'};
const colorMap = {
  amber: 'bg-amber-50 dark:bg-amber-950 text-amber-600 dark:text-amber-400',
  neutral: 'bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400',
  nova:   'bg-nova-50 dark:bg-nova-950 text-nova-600 dark:text-nova-400',
  copper: 'bg-copper-50 dark:bg-copper-950 text-copper-600 dark:text-copper-400',
  green:  'bg-emerald-50 dark:bg-emerald-950 text-emerald-600 dark:text-emerald-400',
  red:    'bg-red-50 dark:bg-red-950 text-red-600 dark:text-red-400',
  blue:   'bg-blue-50 dark:bg-blue-950 text-blue-600 dark:text-blue-400',
};

export default function MetricCard({
  title, value, subtitle, icon: Icon, trend, trendLabel, color = 'nova', statusTone,
}: MetricCardProps) {
  return (
    <div className="card p-5">
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <p className="text-xs font-mono font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400 mb-2">
            {title}
          </p>
          <p className={clsx('text-2xl font-display font-semibold tabular-nums', statusTone ? statusText[statusTone] : 'text-gray-900 dark:text-white')}>
            {value}
          </p>
          {subtitle && (
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{subtitle}</p>
          )}
          {trend !== undefined && (
            <div className="flex items-center gap-1 mt-2">
              {trend > 0 ? (
                <TrendingUp className="w-3.5 h-3.5 text-emerald-500" />
              ) : trend < 0 ? (
                <TrendingDown className="w-3.5 h-3.5 text-red-500" />
              ) : (
                <Minus className="w-3.5 h-3.5 text-gray-400" />
              )}
              <span
                className={clsx(
                  'text-xs font-medium',
                  trend > 0 && 'text-emerald-600 dark:text-emerald-400',
                  trend < 0 && 'text-red-600 dark:text-red-400',
                  trend === 0 && 'text-gray-500',
                )}
              >
                {trend > 0 ? '+' : ''}{trend}%
              </span>
              {trendLabel && (
                <span className="text-xs text-gray-400 ml-1">{trendLabel}</span>
              )}
            </div>
          )}
        </div>
        {Icon && (
          <div className={clsx('w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0', colorMap[color])}>
            <Icon className="w-5 h-5" />
          </div>
        )}
      </div>
    </div>
  );
}
