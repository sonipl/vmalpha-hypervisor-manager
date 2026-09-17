import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { aiAPI } from '@/services/api';
import DataTable, { Column } from '@/components/common/DataTable';
import StatusBadge from '@/components/common/StatusBadge';
import MetricCard from '@/components/common/MetricCard';
import type { AIRecommendation } from '@/types';
import {
  Brain, Zap, TrendingUp, Shield, AlertTriangle, Check,
  X, Send, Bot, Lightbulb,
  ArrowRightLeft, Cpu, Activity,
} from 'lucide-react';
import toast from 'react-hot-toast';

export default function AIConsolePage() {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<'recommendations' | 'policies' | 'chat' | 'migration'>('recommendations');
  const [chatInput, setChatInput] = useState('');
  const [chatHistory, setChatHistory] = useState<{ role: string; content: string }[]>([
    { role: 'assistant', content: 'Hello! I\'m NovaMind, your AI infrastructure assistant. Ask me about VM performance, capacity planning, security recommendations, or anything about your VM Alpha Manager platform.' },
  ]);

  const { data: recs, isError: recommendationsError, isPending: recommendationsLoading } = useQuery({
    queryKey: ['ai-recommendations'],
    queryFn: () => aiAPI.listRecommendations().then((r) => r.data),
  });

  const { data: policies, isError: policiesError, isPending: policiesLoading } = useQuery({
    queryKey: ['ai-policies'],
    queryFn: () => aiAPI.listPolicies().then((r) => r.data),
  });

  const applyMutation = useMutation({
    mutationFn: (id: string) => aiAPI.applyRecommendation(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['ai-recommendations'] }); toast.success('Recommendation applied'); },
  });

  const dismissMutation = useMutation({
    mutationFn: (id: string) => aiAPI.dismissRecommendation(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['ai-recommendations'] }); toast.success('Recommendation dismissed'); },
  });

  const handleChat = async () => {
    if (!chatInput.trim()) return;
    const msg = chatInput;
    setChatInput('');
    setChatHistory((h) => [...h, { role: 'user', content: msg }]);
    try {
      const { data } = await aiAPI.chat(msg);
      setChatHistory((h) => [...h, { role: 'assistant', content: (data as { response: string }).response || 'I\'ll analyze that for you. This feature requires the NovaMind ML service to be running.' }]);
    } catch {
      setChatHistory((h) => [...h, { role: 'assistant', content: 'The NovaMind ML service is not available. Please check the AI engine configuration.' }]);
    }
  };

  const recData = recommendationsError ? [] : recs?.data ?? [];
  const policyData = policiesError ? [] : policies ?? [];

  const typeIcons: Record<string, React.ElementType> = {
    rightsizing: Cpu, placement: ArrowRightLeft, security: Shield,
    cost: TrendingUp, capacity: Activity, anomaly: AlertTriangle,
  };

  const recColumns: Column<AIRecommendation>[] = [
    {
      key: 'type', label: '', width: '40px',
      render: (r) => {
        const Icon = typeIcons[r.type] ?? Lightbulb;
        return <Icon className={`w-4 h-4 ${r.severity === 'critical' ? 'text-red-500' : r.severity === 'warning' ? 'text-amber-500' : 'text-nova-500'}`} />;
      },
    },
    {
      key: 'title', label: 'Recommendation', sortable: true,
      render: (r) => (
        <div>
          <div className="font-medium text-gray-900 dark:text-white">{r.title}</div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 line-clamp-1">{r.description}</div>
        </div>
      ),
    },
    { key: 'type', label: 'Type', width: '100px', render: (r) => <span className="text-xs font-mono bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded capitalize">{r.type}</span> },
    { key: 'severity', label: 'Severity', width: '90px', render: (r) => <StatusBadge status={r.severity} /> },
    {
      key: 'confidence', label: 'Confidence', width: '100px',
      render: (r) => (
        <div className="flex items-center gap-1.5">
          <div className="w-12 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <div className="h-full bg-copper-500 rounded-full" style={{ width: `${(r.confidence ?? 0) * 100}%` }} />
          </div>
          <span className="font-mono text-xs tabular-nums">{((r.confidence ?? 0) * 100).toFixed(0)}%</span>
        </div>
      ),
    },
    { key: 'status', label: 'Status', width: '90px', render: (r) => <StatusBadge status={r.status} /> },
    {
      key: 'actions', label: '', width: '100px',
      render: (r) => r.status === 'pending' ? (
        <div className="flex gap-1">
          <button onClick={(e) => { e.stopPropagation(); applyMutation.mutate(r.id); }}
            className="p-1.5 text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950 rounded" title="Apply">
            <Check className="w-3.5 h-3.5" />
          </button>
          <button onClick={(e) => { e.stopPropagation(); dismissMutation.mutate(r.id); }}
            className="p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 rounded" title="Dismiss">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      ) : null,
    },
  ];

  const tabs = [
    { key: 'recommendations' as const, label: 'Recommendations', icon: Lightbulb },
    { key: 'policies' as const, label: 'Automation Policies', icon: Zap },
    { key: 'chat' as const, label: 'NovaMind Chat', icon: Bot },
    { key: 'migration' as const, label: 'Migration Advisor', icon: ArrowRightLeft },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Brain className="w-6 h-6 text-copper-500" /> AI & Automation
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">NovaMind AI engine for intelligent infrastructure</p>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard title="Pending on This Page" value={recommendationsError || recommendationsLoading ? '—' : recData.filter((r) => r.status === 'pending').length} icon={Lightbulb} color="copper" />
        <MetricCard title="Recorded Applied on This Page" value={recommendationsError || recommendationsLoading ? '—' : recData.filter((r) => r.status === 'applied').length} icon={Check} color="green" />
        <MetricCard title="Automation Policies" value={policiesError || policiesLoading ? '—' : policyData.length} icon={Zap} color="nova" />
        <MetricCard title="Avg. Confidence" value={recData.length ? `${(recData.reduce((sum, r) => sum + r.confidence, 0) / recData.length * 100).toFixed(0)}%` : '—'} icon={Brain} color="blue" />
      </div>

      <div className="border-b border-gray-200 dark:border-gray-800">
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button key={t.key} onClick={() => setTab(t.key)}
              className={`flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                tab === t.key ? 'border-copper-500 text-copper-600 dark:text-copper-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
              }`}>
              <t.icon className="w-4 h-4" /> {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === 'recommendations' && <DataTable columns={recColumns} data={recData} loading={recommendationsLoading} emptyMessage={recommendationsError ? "Recommendations unavailable" : "No recommendations"} />}

      {tab === 'policies' && (
        <div className="space-y-3">
          {policiesError && <p>Policies unavailable.</p>}
          {policiesLoading && <p>Loading policies…</p>}
          {!policiesLoading && !policiesError && !policyData.length && <p>No saved policies.</p>}
          {policyData.map((p) => (
            <div key={p.id} className="card p-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${p.enabled ? 'bg-copper-50 dark:bg-copper-950 text-copper-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-400'}`}>
                  <Zap className="w-4 h-4" />
                </div>
                <div>
                  <div className="font-medium text-gray-900 dark:text-white">{p.name}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{JSON.stringify(p.trigger)}</div>
                </div>
              </div>
              <div className="flex items-center gap-3">
                {p.auto_apply && <span className="text-[10px] font-medium text-copper-600 bg-copper-50 dark:bg-copper-950 px-2 py-0.5 rounded-full">Auto-Apply</span>}
                <StatusBadge status={p.enabled ? 'active' : 'inactive'} />
              </div>
            </div>
          ))}
        </div>
      )}

      {tab === 'chat' && (
        <div className="card flex flex-col h-[500px]">
          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            {chatHistory.map((msg, i) => (
              <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                <div className={`max-w-[70%] rounded-xl px-4 py-2.5 text-sm ${
                  msg.role === 'user'
                    ? 'bg-nova-500 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-white'
                }`}>
                  {msg.role === 'assistant' && (
                    <div className="flex items-center gap-1.5 mb-1 text-xs text-copper-600 dark:text-copper-400 font-medium">
                      <Brain className="w-3 h-3" /> NovaMind
                    </div>
                  )}
                  {msg.content}
                </div>
              </div>
            ))}
          </div>
          <div className="p-3 border-t border-gray-200 dark:border-gray-800">
            <form onSubmit={(e) => { e.preventDefault(); handleChat(); }} className="flex gap-2">
              <input type="text" value={chatInput} onChange={(e) => setChatInput(e.target.value)}
                placeholder="Ask NovaMind about your infrastructure…"
                className="flex-1 px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:ring-2 focus:ring-copper-400 focus:outline-none" />
              <button type="submit" className="px-4 py-2 bg-copper-500 hover:bg-copper-600 text-white rounded-lg text-sm font-medium transition-colors flex items-center gap-1.5">
                <Send className="w-4 h-4" /> Send
              </button>
            </form>
          </div>
        </div>
      )}

      {tab === 'migration' && (
        <div className="card p-8 text-center text-gray-500 dark:text-gray-400">
          <ArrowRightLeft className="w-10 h-10 mx-auto mb-3 opacity-40" />
          <p className="font-medium text-gray-700 dark:text-gray-300">Migration Advisor</p>
          <p className="text-sm mt-1 max-w-md mx-auto">
            Migration assessment is not available yet.
          </p>

        </div>
      )}
    </div>
  );
}
