import React, { useState } from 'react';
import { ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react';
import clsx from 'clsx';

export interface Column<T> {
  key: string;
  label: string;
  sortable?: boolean;
  width?: string;
  render?: (item: T) => React.ReactNode;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyField?: string;
  onRowClick?: (item: T) => void;
  emptyMessage?: string;
  loading?: boolean;
}

export default function DataTable<T extends object>({
  columns, data, keyField = 'id', onRowClick, emptyMessage = 'No data found', loading,
}: DataTableProps<T>) {
  const [sortKey, setSortKey] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');

  const handleSort = (key: string) => {
    if (sortKey === key) {
      setSortDir(sortDir === 'asc' ? 'desc' : 'asc');
    } else {
      setSortKey(key);
      setSortDir('asc');
    }
  };

  const sorted = sortKey
    ? [...data].sort((a, b) => {
        const av = String(a[sortKey as keyof T] ?? '');
        const bv = String(b[sortKey as keyof T] ?? '');
        return sortDir === 'asc' ? av.localeCompare(bv) : bv.localeCompare(av);
      })
    : data;

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                style={col.width ? { width: col.width } : undefined}
                className={clsx(col.sortable && 'cursor-pointer select-none')}
                onClick={() => col.sortable && handleSort(col.key)}
              >
                <div className="flex items-center gap-1">
                  {col.label}
                  {col.sortable && (
                    sortKey === col.key ? (
                      sortDir === 'asc' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />
                    ) : (
                      <ChevronsUpDown className="w-3 h-3 opacity-40" />
                    )
                  )}
                </div>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={columns.length} className="text-center py-12">
                <div className="inline-flex items-center gap-2 text-gray-400">
                  <div className="w-4 h-4 border-2 border-nova-400 border-t-transparent rounded-full animate-spin" />
                  Loading…
                </div>
              </td>
            </tr>
          ) : sorted.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="text-center py-12 text-gray-400">
                {emptyMessage}
              </td>
            </tr>
          ) : (
            sorted.map((item) => (
              <tr
                key={String(item[keyField as keyof T])}
                className={clsx(onRowClick && 'cursor-pointer')}
                onClick={() => onRowClick?.(item)}
              >
                {columns.map((col) => (
                  <td key={col.key}>
                    {col.render ? col.render(item) : String(item[col.key as keyof T] ?? '—')}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
