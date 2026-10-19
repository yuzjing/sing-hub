<!-- frontend/src/lib/components/SettingsDrawer.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    open?: boolean;
    onClose?: (e?: MouseEvent) => void;
  }

  let { open = false, onClose = () => {} }: Props = $props();

  let isSavingSettings = $state(false);
  let linterEnabled = $state(true);

  $effect(() => {
    if (open) {
      linterEnabled = ws.systemSettings.singbox_linter_enabled !== 'false';
      if (!ws.systemSettings.singbox_update_channel) ws.systemSettings.singbox_update_channel = 'all';
      if (!ws.systemSettings.singbox_update_interval) ws.systemSettings.singbox_update_interval = '24';
      ws.fetchUpdaterStatus();
    }
  });

  async function handleToggleLogLevel() {
    const next = ws.debugLogLevel === 'INFO' ? 'DEBUG' : 'INFO';
    await ws.setLogLevel(next);
    ws.showToast('日志级别已切换为: ' + next, 'success');
  }

  async function handleTriggerUpdate() {
    await api.triggerUpdate();
    ws.showToast('已下发后台更新任务，请稍候...', 'info');
    ws.fetchUpdaterStatus();
  }

  async function handleSaveSettings() {
    isSavingSettings = true;
    try {
      ws.systemSettings.singbox_linter_enabled = String(linterEnabled);
      await ws.saveSettings(ws.systemSettings);
      ws.showToast('系统设置已持久化保存 ✓', 'success');

      // 关键：若修改了语法开关，立即静默重新编译当前方案，让大盘警告框立刻消失/显现！
      if (ws.activeProfileId) {
        ws.triggerCompile();
      }

      if (linterEnabled && ws.systemSettings.singbox_update_channel !== 'disabled') {
        handleTriggerUpdate();
      }
    } catch {
      ws.showToast('保存失败', 'error');
    } finally {
      isSavingSettings = false;
    }
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-xs">
    <div class="w-full max-w-md bg-white dark:bg-zinc-900 h-full shadow-2xl flex flex-col border-l border-zinc-200 dark:border-zinc-800 animate-in slide-in-from-right duration-200">
      
      <div class="flex items-center justify-between border-b border-zinc-200 p-4 dark:border-zinc-800 shrink-0">
        <div class="flex items-center gap-2">
          <Icon name="settings" size={18} color="blue" />
          <h2 class="text-sm font-bold text-zinc-900 dark:text-zinc-100">Sing-Hub 控制中心</h2>
        </div>
        <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={16} />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto p-5 space-y-6">
        
        <!-- Sing-box 核心检测控制区 -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <div>
              <span class="font-bold text-zinc-800 dark:text-zinc-100 block text-sm">Sing-box 官方内核语法检测</span>
              <span class="text-xs text-zinc-400">大盘保存时自动调用官方内核执行深度语义校验</span>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" bind:checked={linterEnabled} class="sr-only peer" />
              <div class="w-10 h-5.5 bg-zinc-200 peer-focus:outline-none rounded-full peer dark:bg-zinc-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:border-zinc-300 after:border after:rounded-full after:h-4.5 after:w-4.5 after:transition-all peer-checked:bg-blue-600"></div>
            </label>
          </div>

          {#if linterEnabled}
            <div class="space-y-3 pt-2 border-t border-zinc-100 dark:border-zinc-800 animate-in fade-in duration-150">
              <div class="grid grid-cols-2 gap-3">
                <label class="block">
                  <span class="block text-xs text-zinc-500 mb-1">更新与校验通道</span>
                  <select bind:value={ws.systemSettings.singbox_update_channel} class="w-full rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 cursor-pointer">
                    <option value="all">双轨模式 (Stable + Alpha)</option>
                    <option value="stable">仅正式版 (Stable)</option>
                    <option value="alpha">仅尝鲜版 (Alpha)</option>
                    <option value="disabled">关闭更新</option>
                  </select>
                </label>
                <label class="block">
                  <span class="block text-xs text-zinc-500 mb-1">自动检查频率</span>
                  <select bind:value={ws.systemSettings.singbox_update_interval} class="w-full rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 cursor-pointer">
                    <option value="24">每天一次</option>
                    <option value="0">仅手动触发</option>
                  </select>
                </label>
              </div>

              <div class="rounded-xl border border-blue-200/60 bg-blue-50/40 p-3.5 dark:border-blue-900/40 dark:bg-blue-950/20 space-y-2">
                <div class="flex items-center justify-between border-b border-blue-100 pb-2 dark:border-blue-900/50">
                  <span class="font-semibold text-blue-800 dark:text-blue-300 text-xs">本地内核库运行状态</span>
                  <button
                    disabled={ws.updaterStatus?.is_running}
                    onclick={handleTriggerUpdate}
                    type="button"
                    class="rounded bg-blue-600 px-2.5 py-1 text-xs font-bold text-white hover:bg-blue-500 cursor-pointer disabled:opacity-50 transition"
                  >
                    {ws.updaterStatus?.is_running ? '拉取中...' : '检查并下载'}
                  </button>
                </div>
                <div class="grid grid-cols-2 gap-2 text-xs font-mono text-blue-700/90 dark:text-blue-400">
                  <div>Stable: <strong class="text-blue-950 dark:text-blue-200">{ws.updaterStatus?.stable_version || '-'}</strong></div>
                  <div>Alpha: <strong class="text-blue-950 dark:text-blue-200">{ws.updaterStatus?.alpha_version || '-'}</strong></div>
                  <div class="col-span-2 text-zinc-500 text-[11px]">上次检查: {ws.updaterStatus?.last_check || '-'}</div>
                  {#if ws.updaterStatus?.last_error}
                    <div class="col-span-2 text-rose-500 wrap-break-word mt-1">{ws.updaterStatus.last_error}</div>
                  {/if}
                </div>
              </div>
            </div>
          {/if}
        </div>

        <!-- 界面外观模式 -->
        <div class="space-y-2 border-t border-zinc-100 pt-5 dark:border-zinc-800">
          <span class="font-bold text-zinc-800 dark:text-zinc-100 block text-sm">界面外观模式 (Theme)</span>
          <div class="grid grid-cols-3 gap-1.5 p-1 rounded-xl bg-zinc-100 dark:bg-zinc-800/70 border border-zinc-200/50 dark:border-zinc-700/50">
            <button
              onclick={() => ws.applyTheme('light')}
              type="button"
              class="py-2 rounded-lg text-xs font-medium transition cursor-pointer flex items-center justify-center gap-1.5 {ws.themeMode === 'light' ? 'bg-white shadow-xs text-zinc-900 font-bold dark:bg-zinc-900 dark:text-white' : 'text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200'}"
            >
              <span>☀️ 浅色</span>
            </button>
            <button
              onclick={() => ws.applyTheme('system')}
              type="button"
              class="py-2 rounded-lg text-xs font-medium transition cursor-pointer flex items-center justify-center gap-1.5 {ws.themeMode === 'system' ? 'bg-white shadow-xs text-zinc-900 font-bold dark:bg-zinc-900 dark:text-white' : 'text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200'}"
            >
              <span>🖥️ 自动</span>
            </button>
            <button
              onclick={() => ws.applyTheme('dark')}
              type="button"
              class="py-2 rounded-lg text-xs font-medium transition cursor-pointer flex items-center justify-center gap-1.5 {ws.themeMode === 'dark' ? 'bg-white shadow-xs text-zinc-900 font-bold dark:bg-zinc-900 dark:text-white' : 'text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200'}"
            >
              <span>🌙 深色</span>
            </button>
          </div>
        </div>

        <!-- 实时排错日志级别 -->
        <div class="rounded-xl border border-zinc-200 p-4 dark:border-zinc-800 space-y-2.5 bg-zinc-50/50 dark:bg-zinc-950/40">
          <div class="flex items-center justify-between">
            <span class="font-bold text-zinc-800 dark:text-zinc-100 text-sm">🐞 实时排错日志级别</span>
            <span class="font-mono text-xs px-2 py-0.5 rounded font-bold {ws.debugLogLevel === 'DEBUG' ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300' : 'bg-zinc-200 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400'}">
              {ws.debugLogLevel}
            </span>
          </div>
          <button
            onclick={handleToggleLogLevel}
            type="button"
            class="w-full rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 py-2 text-xs font-medium text-zinc-700 dark:text-zinc-200 hover:bg-zinc-50 dark:hover:bg-zinc-800 transition cursor-pointer"
          >
            {ws.debugLogLevel === 'INFO' ? '开启详细调试日志 (DEBUG)' : '切回标准模式 (INFO)'}
          </button>
        </div>

        <!-- 本地 SQL 备份时光机 -->
        <div class="rounded-xl border border-zinc-200 p-4 dark:border-zinc-800 space-y-3 bg-zinc-50/50 dark:bg-zinc-950/40">
          <div class="flex items-center justify-between">
            <div>
              <span class="font-bold text-zinc-800 dark:text-zinc-100 block text-sm">本地 SQL 备份时光机</span>
              <span class="text-xs text-zinc-400">持久化导出或热恢复数据</span>
            </div>
            <button
              disabled={ws.isBackupWorking}
              onclick={() => ws.triggerServerBackup()}
              type="button"
              class="rounded-lg bg-blue-600 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-2xs transition disabled:opacity-50"
            >
              {ws.isBackupWorking ? '备份中...' : '+ 立即快照'}
            </button>
          </div>

          <div class="space-y-1.5 max-h-36 overflow-y-auto pr-1">
            {#each ws.serverBackups as b}
              <div class="flex items-center justify-between p-2 rounded-lg bg-white dark:bg-zinc-900 border border-zinc-200/60 dark:border-zinc-800 text-xs font-mono">
                <div class="truncate mr-2">
                  <span class="text-zinc-700 dark:text-zinc-300 block truncate">{b.file_name}</span>
                  <span class="text-[10px] text-zinc-400">{(b.size_bytes / 1024).toFixed(1)} KB</span>
                </div>
                <button
                  disabled={ws.isBackupWorking}
                  onclick={() => ws.restoreFromPath(b.path)}
                  type="button"
                  class="rounded bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400 px-2 py-0.5 text-xs font-medium hover:bg-rose-100 cursor-pointer shrink-0"
                >
                  恢复
                </button>
              </div>
            {:else}
              <span class="text-xs text-zinc-400 italic block py-2 text-center">暂无本地快照</span>
            {/each}
          </div>
        </div>

        <!-- GitOps 异地镜像灾备 -->
        <div class="space-y-3 border-t border-zinc-100 pt-5 dark:border-zinc-800">
          <span class="font-bold text-zinc-800 dark:text-zinc-100 block text-sm">GitOps 异地镜像灾备</span>
          <div class="space-y-2">
            <label class="block">
              <span class="block text-xs text-zinc-400 mb-1">远端 Git 仓库地址</span>
              <input
                bind:value={ws.systemSettings.git_remote_url}
                placeholder="如 http://git.lan/repo.git"
                class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950"
              />
            </label>

            <label class="block">
              <span class="block text-xs text-zinc-400 mb-1">Git 访问令牌 (PAT) - <span class="text-blue-500 font-medium">用于拉取内核与异地推送</span></span>
              <input
                bind:value={ws.systemSettings.git_token}
                placeholder="Personal Access Token"
                type="password"
                class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950"
              />
            </label>
          </div>

          <button
            disabled={isSavingSettings}
            onclick={handleSaveSettings}
            type="button"
            class="w-full rounded-lg bg-zinc-900 dark:bg-white text-white dark:text-zinc-950 py-2 text-xs font-semibold hover:opacity-90 transition cursor-pointer shadow-sm"
          >
            {isSavingSettings ? '保存中...' : '保存系统设置'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}