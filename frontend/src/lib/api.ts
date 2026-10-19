// frontend/src/lib/api.ts
import type {
  Profile,
  Component,
  Module,
  Device,
  CommitRecord,
  NodePingReport,
  UpdaterStatus,
  ValidationReport,
  DeployResult
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
      } catch (e) {
        console.debug('Failed to parse error response', e);
      }
      throw new Error(errMsg);
    }

    if (res.status === 204) {
      return {} as T;
    }

    return res.json();
  }

  // ==================== Components ====================

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

  // ==================== Modules ====================

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

  // ==================== Profiles ====================

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

  // 签名中加入 error?: string，拥抱严格类型
  async compileProfile(id: string): Promise<{ json: string; commitHash: string; validation?: ValidationReport; error?: string }> {
    const res = await fetch(`/api/profiles/${id}/compile`, {
      method: 'POST',
    });

    const rawText = await res.text();

    if (!res.ok) {
      let errMsg = `编译未通过: HTTP ${res.status}`;
      let invalidJson = '';
      try {
        const errObj = JSON.parse(rawText);
        if (errObj.error) errMsg = errObj.error;
        if (errObj.json) invalidJson = errObj.json;
      } catch (e) {
        console.debug('Failed to parse compile error', e);
      }

      // 彻底去除 as any，由于签名允许了 error 字段，这里完美匹配！
      return {
        json: invalidJson,
        commitHash: '',
        error: errMsg
      };
    }

    const commitHash = res.headers.get('X-Commit-Hash') || '';
    try {
      const parsed = JSON.parse(rawText);
      if (typeof parsed.json === 'string') {
        return {
          json: parsed.json,
          commitHash: parsed.commit_hash || commitHash,
          validation: parsed.validation,
        };
      }
      return {
        json: JSON.stringify(parsed, null, 2),
        commitHash,
      };
    } catch {
      return {
        json: rawText,
        commitHash,
      };
    }
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

  // ==================== Devices ====================

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

  async deployDevice(deviceId: string): Promise<DeployResult> {
    const res = await fetch(`/api/devices/${deviceId}/deploy`, { method: 'POST' });
    const data = await res.json();
    if (!res.ok) {
      throw { message: data.error || '部署失败', logs: data.logs || '' };
    }
    return data as DeployResult;
  }

  // ==================== System ====================

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

  async getUpdaterStatus(): Promise<UpdaterStatus> {
    return this.request<UpdaterStatus>('/api/system/updater');
  }

  async triggerUpdate(): Promise<void> {
    return this.request<void>('/api/system/updater/trigger', { method: 'POST' });
  }
}

export const api = new ApiClient();