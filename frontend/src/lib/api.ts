import type { Profile, Component, Device, CommitRecord, NodePingReport } from './types';

// API 请求的基础辅助函数
async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const res = await fetch(endpoint, options);
  if (!res.ok) {
    let errText = await res.text();
    try {
      const parsed = JSON.parse(errText);
      if (parsed.error) errText = parsed.error;
    } catch {
      // ignore
    }
    throw new Error(errText || `HTTP Request Failed: ${res.status}`);
  }
  return res.json();
}

export const api = {
  // ================= 方案 (Profiles) =================
  getProfiles: () => fetchAPI<Profile[]>('/api/profiles'),
  
  getProfile: (id: string) => fetchAPI<Profile>(`/api/profiles/${id}`),
  
  saveProfile: (p: Profile) => 
    fetchAPI<{ status: string }>('/api/profiles', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(p),
    }),
    
  deleteProfile: (id: string) => 
    fetchAPI<{ status: string }>(`/api/profiles/${id}`, { method: 'DELETE' }),

  batchDeleteProfiles: (ids: string[]) =>
    fetchAPI<{ status: string }>('/api/profiles/batch-delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    }),

  compileProfile: async (id: string): Promise<{ json: string; commitHash: string }> => {
    const res = await fetch(`/api/profiles/${id}/compile`);
    if (!res.ok) throw new Error(await res.text());
    const jsonStr = await res.text();
    return {
      json: jsonStr,
      commitHash: res.headers.get('X-Commit-Hash') || '',
    };
  },

  // ================= 组件 (Components) =================
  getComponents: () => fetchAPI<Component[]>('/api/components'),

  saveComponent: (c: Component) =>
    fetchAPI<{ status: string }>('/api/components', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(c),
    }),

  deleteComponent: (id: string) =>
    fetchAPI<{ status: string }>(`/api/components/${id}`, { method: 'DELETE' }),

  batchDeleteComponents: (ids: string[]) =>
    fetchAPI<{ status: string }>('/api/components/batch-delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    }),

  previewComponent: async (payload_tmpl: string, params_json: string, node_id: string): Promise<string> => {
    const res = await fetch('/api/components/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ payload_tmpl, params_json, node_id }),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.text(); // 预览接口直接返回格式化后的 JSON 字符串
  },

  // ================= 节点 (Devices) =================
  getDevices: () => fetchAPI<Device[]>('/api/devices'),

  saveDevice: (d: Device) =>
    fetchAPI<{ status: string }>('/api/devices', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(d),
    }),

  deleteDevice: (id: string) =>
    fetchAPI<{ status: string }>(`/api/devices/${id}`, { method: 'DELETE' }),

  batchDeleteDevices: (ids: string[]) =>
    fetchAPI<{ status: string }>('/api/devices/batch-delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    }),

  pingDevice: (id: string) =>
    fetchAPI<{ online: boolean; latency_ms: number; error?: string }>(`/api/devices/${id}/ping`, { method: 'POST' }),

  pingAllDevices: () =>
    fetchAPI<NodePingReport[]>('/api/devices/ping-all', { method: 'POST' }),

  deployDevice: (id: string) =>
    fetchAPI<{ status: string; verified_hash: string }>(`/api/devices/${id}/deploy`, { method: 'POST' }),

  // ================= 历史与版本 (Git) =================
  getHistory: (profileId: string) => fetchAPI<CommitRecord[]>(`/api/profiles/${profileId}/history`),

  getDiff: (profileId: string, commitHash: string) =>
    fetchAPI<{ diff: string }>(`/api/profiles/${profileId}/diff?commit=${commitHash}`),

  rollback: async (profileId: string, commitHash: string): Promise<string> => {
    const res = await fetch(`/api/profiles/${profileId}/rollback?commit=${commitHash}`, { method: 'POST' });
    if (!res.ok) throw new Error(await res.text());
    return res.text();
  },

  // ================= 系统与运维 (System) =================
  getLogLevel: () => fetchAPI<{ level: string }>('/api/system/log-level'),
  
  setLogLevel: (level: string) =>
    fetchAPI<{ status: string }>('/api/system/log-level', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ level }),
    }),

  getSettings: () => fetchAPI<Record<string, string>>('/api/system/settings'),

  saveSettings: (settings: Record<string, string>) =>
    fetchAPI<{ status: string }>('/api/system/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    }),

  // 获取服务端物理备份列表
  listServerBackups: () => 
    fetchAPI<{ file_name: string; path: string; size_bytes: number; created_at: string }[]>('/api/system/backups'),

  // 触发服务端生成快照
  createServerBackup: () => 
    fetchAPI<{ file_name: string; path: string; commit_hash: string }>('/api/system/backup/create', { method: 'POST' }),

  // 通过绝对路径热恢复
  restoreFromServerPath: (path: string) =>
    fetchAPI<{ status: string; path: string }>('/api/system/restore/path', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    }),
};