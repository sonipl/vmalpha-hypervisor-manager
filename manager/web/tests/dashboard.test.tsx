import React from 'react';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import DashboardPage from '../src/pages/DashboardPage';

function render(client: QueryClient) {
 return renderToStaticMarkup(<QueryClientProvider client={client}><MemoryRouter><DashboardPage /></MemoryRouter></QueryClientProvider>);
}
test('dashboard shows unavailable telemetry and does not invent records while loading', () => {
 const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
 const html = render(client);
 assert.match(html, /Loading inventory/);
 assert.match(html, /CPU: unavailable/);
 for (const fake of ['All Systems Operational', 'vm-web-01', 'vm-db-02', '4.2 TB', 'vs last week']) assert.ok(!html.includes(fake), fake);
 client.clear();
});
test('dashboard consumes nested counts and paginated VM data without sample fallbacks', () => {
 const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
 client.setQueryData(['dashboard-metrics'], { vms: { total: 1, running: 1, stopped: 0, error: 0, migrating: 0, paused: 0, provisioning: 0 }, hosts: { total: 4, ready: 3 }, cluster_health: 'Unknown', telemetry_status: 'unavailable', utilization: { cpu_percent: null, memory_percent: null, storage_percent: null, network_mbps: null } });
 client.setQueryData(['vms-summary'], { data: [{ id: 'test-vm', name: 'Observed test guest', status: 'Running', vcpus: 2, memory_mb: 1024 }], total: 1, page: 1, per_page: 5 });
 const html = render(client);
 assert.match(html, /Observed test guest/);
 assert.match(html, /3 ready in inventory/);
 assert.match(html, /1 running/);
 assert.match(html, /CPU: unavailable/);
 assert.ok(!html.includes('vm-web-01'));
 client.clear();
});
test('dashboard hides cached inventory after a failed refresh', () => {
 const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
 client.setQueryData(['dashboard-metrics'], { vms: { total: 999 } });
 const query = client.getQueryCache().find({ queryKey: ['dashboard-metrics'] });
 query!.setState({ status: 'error', error: new Error('API unavailable') });
 const html = render(client);
 assert.match(html, /Inventory unavailable/);
 assert.ok(!html.includes('999'));
 client.clear();
});
