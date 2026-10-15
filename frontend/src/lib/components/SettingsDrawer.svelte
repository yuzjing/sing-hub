<!-- frontend/src/lib/components/SettingsDrawer.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';

  let { open = false, onClose }: { open: boolean; onClose: () => void } = $props();
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm">
    <div class="w-96 bg-white p-6 shadow-2xl dark:bg-zinc-950 border-l border-zinc-200 dark:border-zinc-800 space-y-6 overflow-y-auto">
      <div class="flex items-center justify-between border-b border-zinc-200 pb-3 dark:border-zinc-800">
        <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100">Sing-Hub 控制中心</h3>
        <button onclick={onClose} class="text-xs text-zinc-400 hover:text-white cursor-pointer">✕ 关 闭</button>
      </div>

      <!-- 1. 外观三态 -->
      <div class="space-y-2 text-xs">
        <span class="font-semibold block text-zinc-700 dark:text-zinc-300">界面外观模式 (Theme)</span>
        <div class="grid grid-cols-3 gap-1 rounded-lg border border-zinc-200 bg-zinc-100 p-1 dark:border-zinc-800 dark:bg-zinc-900">
          <button
            onclick={() => ws.applyTheme('light')}
            class="rounded py-1 text-center font-medium transition cursor-pointer {ws.themeMode === 'light' ? 'bg-white shadow text-zinc-900 dark:bg-zinc-800 dark:text-white' : 'text-zinc-500'}"
          >
            ☀️ 浅色
          </button>
          <button
            onclick={() => ws.applyTheme('system')}
            class="rounded py-1 text-center font-medium transition cursor-pointer {ws.themeMode === 'system' ? 'bg-white shadow text-zinc-900 dark:bg-zinc-800 dark:text-white' : 'text-zinc-500'}"
          >
            🖥️ 自动
          </button>
          <button
            onclick={() => ws.applyTheme('dark')}
            class="rounded py-1 text-center font-medium transition cursor-pointer {ws.themeMode === 'dark' ? 'bg-white shadow text-zinc-900 dark:bg-zinc-800 dark:text-white' : 'text-zinc-500'}"
          >
            🌙 深色
          </button>
        </div>
      </div>

      <!-- 2. 全链路 Debug 日志切换 -->
      <div class="space-y-2 text-xs border-t border-zinc-100 pt-4 dark:border-zinc-800">
        <div class="flex items-center justify-between">
          <span class="font-semibold text-zinc-700 dark:text-zinc-300">🐞 详细排错日志 (Debug Level)</span>
          <span class="font-mono text-[10px] px-1.5 py-0.5 rounded {ws.debugLogLevel === 'DEBUG' ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-400 font-bold' : 'bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400'}">
            {ws.debugLogLevel}
          </span>
        </div>
        <button
          onclick={() => ws.setLogLevel(ws.debugLogLevel === 'DEBUG' ? 'INFO' : 'DEBUG')}
          class="w-full rounded border border-zinc-300 bg-zinc-50 py-1.5 font-medium text-xs text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer"
        >
          {ws.debugLogLevel === 'DEBUG' ? '切回生产日志 (INFO)' : '开启详细调试日志 (DEBUG)'}
        </button>
      </div>

      <!-- 3. 本地历史 SQL 快照中枢 (完全消灭手动输入绝对路径) -->
      <div class="space-y-3 text-xs border-t border-zinc-100 pt-4 dark:border-zinc-800">
        <div class="flex items-center justify-between">
          <div>
            <span class="font-semibold text-zinc-700 dark:text-zinc-300 block">本地 SQL 快照时光机</span>
            <span class="text-[10px] text-zinc-400">自动由服务端持久化管理</span>
          </div>
          <button
            disabled={ws.isBackupWorking}
            onclick={() => ws.triggerServerBackup()}
            class="rounded bg-blue-600 px-2.5 py-1 text-white hover:bg-blue-500 cursor-pointer disabled:opacity-40"
          >
            {ws.isBackupWorking ? '生成中...' : '+ 立即快照'}
          </button>
        </div>

        <div class="max-h-48 overflow-y-auto space-y-1.5 rounded-lg border border-zinc-200 p-2 dark:border-zinc-800 bg-zinc-50/50 dark:bg-zinc-900/50">
          {#if ws.serverBackups.length === 0}
            <span class="text-[11px] text-zinc-400 block p-2 text-center">暂无本地历史快照</span>
          {:else}
            {#each ws.serverBackups as b (b.file_name)}
              <div class="flex items-center justify-between p-2 rounded bg-white shadow-sm dark:bg-zinc-900 border border-zinc-100 dark:border-zinc-800/80 text-[11px]">
                <div class="truncate flex-1 mr-2">
                  <div class="font-mono text-zinc-800 dark:text-zinc-200 truncate">{b.file_name}</div>
                  <span class="text-[10px] text-zinc-400">{b.created_at} ({(b.size_bytes / 1024).toFixed(1)} KB)</span>
                </div>
                <button
                  onclick={() => ws.restoreFromPath(b.path)}
                  class="rounded bg-rose-50 px-2.5 py-1 text-rose-600 font-semibold hover:bg-rose-100 dark:bg-rose-950/40 dark:text-rose-400 cursor-pointer"
                >
                  原地恢复
                </button>
              </div>
            {/each}
          {/if}
        </div>
      </div>

      <!-- 4. Git 远端异地灾备 -->
      <div class="space-y-3 text-xs border-t border-zinc-100 pt-4 dark:border-zinc-800">
        <span class="font-semibold block text-zinc-700 dark:text-zinc-300">GitOps 异地镜像灾备</span>
        <label class="block">
          <span class="text-[11px] text-zinc-400 block mb-0.5">远端仓库 URL</span>
          <input
            bind:value={ws.systemSettings.git_remote_url}
            onchange={() => ws.saveSettings({ git_remote_url: ws.systemSettings.git_remote_url })}
            placeholder="http://10.144.0.5:3000/sana/configs.git"
            class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-900"
          />
        </label>
        <label class="block">
          <span class="text-[11px] text-zinc-400 block mb-0.5">访问令牌 (PAT)</span>
          <input
            type="password"
            bind:value={ws.systemSettings.git_token}
            onchange={() => ws.saveSettings({ git_token: ws.systemSettings.git_token })}
            placeholder="gitea_pat_xxxx"
            class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-900"
          />
        </label>
      </div>
    </div>
  </div>
{/if}