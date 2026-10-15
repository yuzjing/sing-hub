// frontend/src/lib/api.ts
import type { Component, Profile, Device, CommitRecord, NodePingReport } from './types';

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `请求失败: ${res.status}`);
  }
  return res.json();
}

export const api = {
  getComponents: () => req<Component[]>('/api/components'),
  saveComponent: (c: Component) => req<{ status: string }>('/api/components', {
    method: 'POST',
    body: JSON.stringify(c),
  }),
  deleteComponent: (id: string) => req<{ status: string }>(`/api/components/${id}`, {
    method: 'DELETE',
  }),
  batchDeleteComponents: (ids: string[]) => req<{ status: string }>('/api/components/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  }),
  
  // 【关键修复】：这里必须带上 node_id: string = ''
  previewComponent: async (payload_tmpl: string, params_json: string, node_id: string = ''): Promise<string> => {
    const res = await fetch('/api/components/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ payload_tmpl, params_json, node_id }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: '渲染失败' }));
      throw new Error(err.error);
    }
    return res.text();
  },

  getProfiles: () => req<Profile[]>('/api/profiles'),
  getProfile: (id: string) => req<Profile>(`/api/profiles/${id}`),
  saveProfile: (p: Profile) => req<{ status: string }>('/api/profiles', {
    method: 'POST',
    body: JSON.stringify(p),
  }),
  deleteProfile: (id: string) => req<{ status: string }>(`/api/profiles/${id}`, {
    method: 'DELETE',
  }),
  batchDeleteProfiles: (ids: string[]) => req<{ status: string }>('/api/profiles/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  }),
  compileProfile: async (id: string): Promise<{ json: string; commitHash: string }> => {
    const res = await fetch(`/api/profiles/${id}/compile`);
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: '编译失败' }));
      throw new Error(err.error);
    }
    const commitHash = res.headers.get('X-Commit-Hash') || '';
    const json = await res.text();
    return { json, commitHash };
  },

  getHistory: (profileId: string) => req<CommitRecord[]>(`/api/profiles/${profileId}/history`),
  getDiff: (profileId: string, commit: string) => req<{ diff: string }>(`/api/profiles/${profileId}/diff?commit=${commit}`),
  rollback: async (profileId: string, commit: string): Promise<string> => {
    const res = await fetch(`/api/profiles/${profileId}/rollback?commit=${commit}`, { method: 'POST' });
    if (!res.ok) throw new Error('回滚失败');
    return res.text();
  },

  getDevices: () => req<Device[]>('/api/devices'),
  saveDevice: (d: Device) => req<{ status: string }>('/api/devices', {
    method: 'POST',
    body: JSON.stringify(d),
  }),
  deleteDevice: (id: string) => req<{ status: string }>(`/api/devices/${id}`, {
    method: 'DELETE',
  }),
  batchDeleteDevices: (ids: string[]) => req<{ status: string }>('/api/devices/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  }),
  pingDevice: (id: string) => req<{ online: boolean; latency_ms: number; error?: string }>(`/api/devices/${id}/ping`, { method: 'POST' }),
  pingAllDevices: () => req<NodePingReport[]>('/api/devices/ping-all', { method: 'POST' }),
  deployDevice: (id: string) => req<{ status: string; verified_hash: string }>(`/api/devices/${id}/deploy`, { method: 'POST' }),

  sqlBackupDownloadUrl: '/api/system/backup/sql',
  restoreSql: async (sqlText: string) => {
    const res = await fetch('/api/system/restore/sql', {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      body: sqlText,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: '恢复失败' }));
      throw new Error(err.error);
    }
    return res.json();
  },

  getSettings: () => req<Record<string, string>>('/api/system/settings'),
  saveSettings: (settings: Record<string, string>) => req<{ status: string }>('/api/system/settings', {
    method: 'POST',
    body: JSON.stringify(settings),
  }),
  getLogLevel: () => req<{ level: string }>('/api/system/log-level'),
  setLogLevel: (level: string) => req<{ status: string }>('/api/system/log-level', {
    method: 'POST',
    body: JSON.stringify({ level }),
  }),
};