// frontend/src/lib/workspace.svelte.ts
import { api } from './api';
import type { Profile, Component, Device, ProfileComponentBinding, CommitRecord, NodePingReport } from './types';

export class WorkspaceState {
  profiles = $state<Profile[]>([]);
  components = $state<Component[]>([]);
  devices = $state<Device[]>([]);

  currentWorkspace = $state<'library' | 'studio' | 'fleet'>('studio');
  showSettingsDrawer = $state(false);
  isSidebarExpanded = $state(false);

  nodePingStatuses = $state<Record<string, { loading: boolean; latency?: number; online?: boolean; error?: string }>>({});
  isPingAllLoading = $state(false);

  activeProfileId = $state<string>('');
  compiledJSON = $state<string>('// 点击编译方案查看产物');
  activeCommitHash = $state<string>('');
  compileError = $state<string>('');
  isCompiling = $state<boolean>(false);

  showTimeline = $state(false);
  commitHistory = $state<CommitRecord[]>([]);
  currentDiff = $state<string>('');

  serverBackups = $state<{ file_name: string; path: string; size_bytes: number; created_at: string }[]>([]);
  isBackupWorking = $state(false);

  systemSettings = $state<Record<string, string>>({
    git_remote_url: '',
    git_branch: 'main',
    git_token: '',
    ssh_timeout: '5',
  });

  themeMode = $state<'system' | 'dark' | 'light'>('system');
  isDark = $state(true);
  debugLogLevel = $state<'INFO' | 'DEBUG'>('INFO');
  toastMessage = $state<{ text: string; type: 'success' | 'error' | 'info' } | null>(null);

  activeProfile = $derived.by(() => {
    return this.profiles.find((p: Profile) => p.id === this.activeProfileId) || null;
  });

  resolvedBindings = $derived.by(() => {
    if (!this.activeProfile) return [];
    const compMap = Object.fromEntries(this.components.map((c: Component) => [c.id, c]));
    return (this.activeProfile.components || []).map((b: ProfileComponentBinding) => {
      const realComp = compMap[b.component_id];
      return {
        binding: b,
        component: realComp || {
          id: b.component_id,
          name: `积木 [${b.component_id}]`,
          category: 'io',
          scope_tag: 'universal',
          mount_path: 'inbounds',
          strategy: 'append',
          payload_tmpl: '{}',
          description: '',
        },
      };
    });
  });

  switchWorkspace(wsName: 'library' | 'studio' | 'fleet') {
    this.currentWorkspace = wsName;
    this.showSettingsDrawer = false;
    this.showTimeline = false;
  }

  showToast(text: string, type: 'success' | 'error' | 'info' = 'info') {
    this.toastMessage = { text, type };
    setTimeout(() => {
      if (this.toastMessage?.text === text) this.toastMessage = null;
    }, 1400);
  }

  initTheme() {
    const saved = localStorage.getItem('sing-hub-theme') as 'system' | 'dark' | 'light' | null;
    this.applyTheme(saved || 'system');
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (this.themeMode === 'system') this.applyTheme('system');
    });

    const savedSidebar = localStorage.getItem('sing-hub-sidebar-expanded');
    if (savedSidebar !== null) {
      this.isSidebarExpanded = savedSidebar === 'true';
    }
  }

  toggleSidebar() {
    this.isSidebarExpanded = !this.isSidebarExpanded;
    localStorage.setItem('sing-hub-sidebar-expanded', String(this.isSidebarExpanded));
  }

  applyTheme(mode: 'system' | 'dark' | 'light') {
    this.themeMode = mode;
    localStorage.setItem('sing-hub-theme', mode);
    this.isDark = mode === 'system' ? window.matchMedia('(prefers-color-scheme: dark)').matches : mode === 'dark';
    if (this.isDark) {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }

  async init() {
    this.initTheme();
    await Promise.all([
      this.loadProfiles(),
      this.loadComponents(),
      this.loadDevices(),
      this.loadSettings(),
      this.loadServerBackups(),
      this.fetchLogLevel()
    ]);
    if (this.profiles.length > 0 && !this.activeProfileId) {
      this.selectProfile(this.profiles[0].id);
    }
  }

  async loadSettings() {
    try {
      this.systemSettings = await api.getSettings();
    } catch (e) {
      console.debug('No custom settings found', e);
    }
  }

  async saveSettings(newSettings: Record<string, string>) {
    this.systemSettings = { ...this.systemSettings, ...newSettings };
    await api.saveSettings(this.systemSettings);
  }

  async fetchLogLevel() {
    try {
      const res = await api.getLogLevel();
      this.debugLogLevel = res.level as 'INFO' | 'DEBUG';
    } catch (e) {
      console.debug('Failed to fetch log level', e);
      this.debugLogLevel = 'INFO';
    }
  }

  async setLogLevel(level: 'INFO' | 'DEBUG') {
    this.debugLogLevel = level;
    await api.setLogLevel(level);
  }

  async loadProfiles() {
    this.profiles = await api.getProfiles();
  }

  async loadComponents() {
    this.components = await api.getComponents();
  }

  async loadDevices() {
    this.devices = await api.getDevices();
  }

  async loadServerBackups() {
    try {
      this.serverBackups = await api.listServerBackups();
    } catch (e) {
      console.debug('Failed to load backups', e);
      this.serverBackups = [];
    }
  }

  async triggerServerBackup() {
    this.isBackupWorking = true;
    try {
      const res = await api.createServerBackup();
      this.showToast(`快照已生成: ${res.file_name}`, 'success');
      await this.loadServerBackups();
    } catch (e: unknown) {
      this.showToast(e instanceof Error ? e.message : '备份失败', 'error');
    } finally {
      this.isBackupWorking = false;
    }
  }

  async restoreFromPath(path: string) {
    this.isBackupWorking = true;
    try {
      await api.restoreFromServerPath(path);
      this.showToast('数据库已成功热恢复！', 'success');
      await this.init();
    } catch (e: unknown) {
      this.showToast(e instanceof Error ? e.message : '恢复失败', 'error');
    } finally {
      this.isBackupWorking = false;
    }
  }

  async selectProfile(id: string) {
    this.activeProfileId = id;
    try {
      const full = await api.getProfile(id);
      const idx = this.profiles.findIndex((p: Profile) => p.id === id);
      if (idx !== -1) {
        this.profiles[idx] = full;
      }
    } catch (e) {
      console.debug('Failed to load profile details', e);
    }
    this.triggerCompile();
  }

  async triggerCompile() {
    if (!this.activeProfileId) return;
    this.isCompiling = true;
    this.compileError = '';
    try {
      const res = await api.compileProfile(this.activeProfileId);
      this.compiledJSON = res.json;
      this.activeCommitHash = res.commitHash;
    } catch (e: unknown) {
      this.compileError = e instanceof Error ? e.message : '编译错误';
    } finally {
      this.isCompiling = false;
    }
  }

  async pingNode(nodeId: string) {
    this.nodePingStatuses[nodeId] = { loading: true };
    try {
      const res = await api.pingDevice(nodeId);
      this.nodePingStatuses[nodeId] = {
        loading: false,
        online: res.online,
        latency: res.latency_ms,
        error: res.error,
      };
    } catch (e: unknown) {
      this.nodePingStatuses[nodeId] = {
        loading: false,
        online: false,
        error: e instanceof Error ? e.message : '探活异常',
      };
    }
  }

  async pingAllNodes() {
    this.isPingAllLoading = true;
    try {
      const reports: NodePingReport[] = await api.pingAllDevices();
      reports.forEach(r => {
        this.nodePingStatuses[r.device_id] = {
          loading: false,
          online: r.online,
          latency: r.latency_ms,
          error: r.error,
        };
      });
    } catch (e: unknown) {
      console.error('一键探活异常', e);
    } finally {
      this.isPingAllLoading = false;
    }
  }

  async openTimeline() {
    if (!this.activeProfileId) return;
    this.showTimeline = true;
    this.commitHistory = await api.getHistory(this.activeProfileId);
  }

  async viewDiff(commit: string) {
    if (!this.activeProfileId) return;
    const res = await api.getDiff(this.activeProfileId, commit);
    this.currentDiff = res.diff;
  }

  async rollbackTo(commit: string) {
    if (!this.activeProfileId) return;
    this.compiledJSON = await api.rollback(this.activeProfileId, commit);
    this.activeCommitHash = commit;
    this.showTimeline = false;
    await this.loadProfiles();
  }

  async moveBinding(index: number, direction: 'up' | 'down') {
    if (!this.activeProfile) return;
    const bindings = [...this.activeProfile.components];
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= bindings.length) return;

    const temp = bindings[index];
    bindings[index] = bindings[targetIndex];
    bindings[targetIndex] = temp;

    bindings.forEach((b: ProfileComponentBinding, idx: number) => {
      b.order_index = idx + 1;
    });

    this.activeProfile.components = bindings;
    await api.saveProfile(this.activeProfile);
    this.triggerCompile();
  }

  async attachComponents(componentIds: string[]) {
    if (!this.activeProfile) return;
    const existingIds = (this.activeProfile.components || []).map(b => b.component_id);
    const toAdd = componentIds.filter(id => !existingIds.includes(id));
    if (toAdd.length === 0) return;

    let maxOrder = (this.activeProfile.components || []).reduce(
      (max: number, b) => Math.max(max, b.order_index),
      0
    );

    const newBindings = [...(this.activeProfile.components || [])];
    for (const id of toAdd) {
      maxOrder++;
      newBindings.push({
        component_id: id,
        order_index: maxOrder,
        params_json: '{}',
      });
    }

    this.activeProfile.components = newBindings;
    await api.saveProfile(this.activeProfile);

    await this.loadProfiles();
    const currentId = this.activeProfileId;
    this.activeProfileId = '';
    queueMicrotask(() => {
      this.activeProfileId = currentId;
      this.triggerCompile();
    });
  }

  async detachComponent(componentId: string) {
    if (!this.activeProfile) return;
    this.activeProfile.components = this.activeProfile.components.filter(
      (b: ProfileComponentBinding) => b.component_id !== componentId
    );
    await api.saveProfile(this.activeProfile);

    await this.loadProfiles();
    const currentId = this.activeProfileId;
    this.activeProfileId = '';
    queueMicrotask(() => {
      this.activeProfileId = currentId;
      this.triggerCompile();
    });
  }

  async updateBindingSingleParam(componentId: string, paramKey: string, paramVal: string | number | boolean) {
    if (!this.activeProfile) return;
    const item = this.activeProfile.components.find((b: ProfileComponentBinding) => b.component_id === componentId);
    if (!item) return;

    let currentObj: Record<string, string | number | boolean> = {};
    try {
      const parsed = JSON.parse(item.params_json || '{}');
      if (typeof parsed === 'object' && parsed !== null) {
        currentObj = parsed;
      }
    } catch (e) {
      console.debug('Failed to parse existing params_json', e);
    }

    currentObj[paramKey] = paramVal;
    item.params_json = JSON.stringify(currentObj);

    await api.saveProfile(this.activeProfile);
    this.triggerCompile();
  }

  async batchDeleteComponents(ids: string[]) {
    await api.batchDeleteComponents(ids);
    await this.loadComponents();
    if (this.activeProfileId) this.triggerCompile();
  }

  async batchDeleteProfiles(ids: string[]) {
    await api.batchDeleteProfiles(ids);
    await this.loadProfiles();
    if (ids.includes(this.activeProfileId)) {
      this.activeProfileId = this.profiles[0]?.id || '';
    }
    if (this.activeProfileId) this.triggerCompile();
  }

  async batchDeleteDevices(ids: string[]) {
    await api.batchDeleteDevices(ids);
    await this.loadDevices();
  }
}

export const ws = new WorkspaceState();