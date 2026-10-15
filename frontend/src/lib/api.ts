// frontend/src/lib/api.ts
import type {
  Profile,
  Component,
  Module,
  Device,
  CommitRecord,
  NodePingReport,
} from './types';

class ApiClient {
  private async request<T>(url: string, init?: RequestInit): Promise<T> {
    const res = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...init?.headers,
      },
      ...init,
    });

    if (!res.ok) {
      let errMsg = `请求失败: HTTP ${res.status}`;
      try {
        const errObj = await res.json();
        if (errObj && errObj.error) {
          errMsg = errObj.error;
        }
      } catch {
        // 无 JSON 则回退
      }
      throw new Error(errMsg);
    }

    if (res.status === 204) {
      return {} as T;
    }

    return res.json();
  }

  // ==================== 1. Components (工坊母版) ====================

  async getComponents(): Promise<Component[]> {
    return this.request<Component[]>('/api/components');
  }

  async saveComponent(c: Component): Promise<void> {
    return this.request<void>('/api/components', {
      method: 'POST',
      body: JSON.stringify(c),
    });
  }

  async deleteComponent(id: string): Promise<void> {
    return this.request<void>(`/api/components/${id}`, {
      method: 'DELETE',
    });
  }

  async batchDeleteComponents(ids: string[]): Promise<void> {
    return this.request<void>('/api/components/batch-delete', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
  }

  async previewComponent(tmpl: string, paramsJson: string, nodeId: string = ''): Promise<string> {
    // 严格匹配后端的 payload_tmpl 与 params_json 字段
    const data = await this.request<{ json: string }>('/api/components/preview', {
      method: 'POST',
      body: JSON.stringify({
        payload_tmpl: tmpl,
        params_json: paramsJson,
        node_id: nodeId,
      }),
    });
    return data.json;
  }

  // ==================== 2. Modules (成品积木库) ====================

  async getModules(): Promise<Module[]> {
    return this.request<Module[]>('/api/modules');
  }

  async saveModule(m: Module): Promise<void> {
    return this.request<void>('/api/modules', {
      method: 'POST',
      body: JSON.stringify(m),
    });
  }

  async deleteModule(id: string): Promise<void> {
    return this.request<void>(`/api/modules/${id}`, {
      method: 'DELETE',
    });
  }

  async batchDeleteModules(ids: string[]): Promise<void> {
    return this.request<void>('/api/modules/batch-delete', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
  }

  // ==================== 3. Profiles (大盘方案) ====================

  async getProfiles(): Promise<Profile[]> {
    return this.request<Profile[]>('/api/profiles');
  }

  async getProfile(id: string): Promise<Profile> {
    return this.request<Profile>(`/api/profiles/${id}`);
  }

  async saveProfile(p: Profile): Promise<void> {
    return this.request<void>('/api/profiles', {
      method: 'POST',
      body: JSON.stringify(p),
    });
  }

  async deleteProfile(id: string): Promise<void> {
    return this.request<void>(`/api/profiles/${id}`, {
      method: 'DELETE',
    });
  }

  async batchDeleteProfiles(ids: string[]): Promise<void> {
    return this.request<void>('/api/profiles/batch-delete', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
  }

  // 编译直接接收返回的 JSON，并从响应头中捕获 Git Commit Hash
  async compileProfile(id: string): Promise<{ json: string; commitHash: string }> {
    const res = await fetch(`/api/profiles/${id}/compile`, {
      method: 'POST',
    });
    if (!res.ok) {
      let errMsg = `编译失败: HTTP ${res.status}`;
      try {
        const errObj = await res.json();
        if (errObj && errObj.error) errMsg = errObj.error;
      } catch {}
      throw new Error(errMsg);
    }
    const commitHash = res.headers.get('X-Commit-Hash') || '';
    const json = await res.text();
    return { json, commitHash };
  }

  async getHistory(profileId: string): Promise<CommitRecord[]> {
    return this.request<CommitRecord[]>(`/api/profiles/${profileId}/history`);
  }

  async getDiff(profileId: string, commit: string): Promise<{ diff: string }> {
    return this.request<{ diff: string }>(`/api/profiles/${profileId}/diff?commit=${commit}`);
  }

  async rollback(profileId: string, commit: string): Promise<string> {
    const res = await this.request<{ json: string }>(`/api/profiles/${profileId}/rollback`, {
      method: 'POST',
      body: JSON.stringify({ commit }),
    });
    return res.json;
  }

  // ==================== 4. Devices (节点运维) ====================

  async getDevices(): Promise<Device[]> {
    return this.request<Device[]>('/api/devices');
  }

  async saveDevice(d: Device): Promise<void> {
    return this.request<void>('/api/devices', {
      method: 'POST',
      body: JSON.stringify(d),
    });
  }

  async deleteDevice(id: string): Promise<void> {
    return this.request<void>(`/api/devices/${id}`, {
      method: 'DELETE',
    });
  }

  async batchDeleteDevices(ids: string[]): Promise<void> {
    return this.request<void>('/api/devices/batch-delete', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    });
  }

  async pingDevice(id: string): Promise<{ online: boolean; latency_ms: number; error?: string }> {
    return this.request<{ online: boolean; latency_ms: number; error?: string }>(`/api/devices/${id}/ping`, {
      method: 'POST',
    });
  }

  async pingAllDevices(): Promise<NodePingReport[]> {
    return this.request<NodePingReport[]>('/api/devices/ping-all', {
      method: 'POST',
    });
  }

  async deployDevice(deviceId: string): Promise<{ verified_hash: string }> {
    return this.request<{ verified_hash: string }>(`/api/devices/${deviceId}/deploy`, {
      method: 'POST',
    });
  }

  // ==================== 5. System Settings & Backups (对齐 /api/system/ 路径) ====================

  async getSettings(): Promise<Record<string, string>> {
    return this.request<Record<string, string>>('/api/system/settings');
  }

  async saveSettings(settings: Record<string, string>): Promise<void> {
    return this.request<void>('/api/system/settings', {
      method: 'POST',
      body: JSON.stringify(settings),
    });
  }

  async getLogLevel(): Promise<{ level: string }> {
    return this.request<{ level: string }>('/api/system/log-level');
  }

  async setLogLevel(level: 'INFO' | 'DEBUG'): Promise<void> {
    return this.request<void>('/api/system/log-level', {
      method: 'POST',
      body: JSON.stringify({ level }),
    });
  }

  async listServerBackups(): Promise<{ file_name: string; path: string; size_bytes: number; created_at: string }[]> {
    return this.request<{ file_name: string; path: string; size_bytes: number; created_at: string }[]>('/api/system/backups');
  }

  async createServerBackup(): Promise<{ file_name: string; path: string }> {
    return this.request<{ file_name: string; path: string }>('/api/system/backup/create', {
      method: 'POST',
    });
  }

  async restoreFromServerPath(path: string): Promise<void> {
    return this.request<void>('/api/system/restore/path', {
      method: 'POST',
      body: JSON.stringify({ path }),
    });
  }
}

export const api = new ApiClient();