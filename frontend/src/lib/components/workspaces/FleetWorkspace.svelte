<!-- frontend/src/lib/components/workspaces/FleetWorkspace.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Device } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  let {
    isDeviceManageMode = $bindable(false),
    selectedDeviceIds = $bindable<string[]>([]),
    onEditDevice = () => {},
    onCloneDevice = () => {},
    triggerConfirm = () => {}
  } = $props<{
    isDeviceManageMode?: boolean;
    selectedDeviceIds?: string[];
    onEditDevice?: (dev: Device) => void;
    onCloneDevice?: (dev: Device) => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }>();

  let isDeployingId = $state<string>('');
  let editingPathDevId = $state<string>('');
  let tempRemotePath = $state('');

  // 统计概览指标
  let onlineCount = $derived.by(() => {
    return ws.devices.filter(d => ws.nodePingStatuses[d.id]?.online === true).length;
  });
  let blockedCount = $derived.by(() => {
    return ws.devices.filter(d => ws.nodePingStatuses[d.id]?.online === false).length;
  });
  let pushReadyCount = $derived.by(() => {
    return ws.devices.filter(d => d.enable_push && !!d.target_profile_id).length;
  });

  async function handleDeploy(dev: Device) {
    if (!dev.enable_push || !dev.target_profile_id) return;
    isDeployingId = dev.id;
    try {
      const res = await api.deployDevice(dev.id);
      ws.showToast(`推送成功！远程校验哈希: ${res.verified_hash.slice(0, 8)} ✓`, 'success');
    } catch (e: unknown) {
      ws.showToast(e instanceof Error ? e.message : '推送失败', 'error');
    } finally {
      isDeployingId = '';
    }
  }

  async function saveRemotePath(dev: Device) {
    if (tempRemotePath.trim() && tempRemotePath !== dev.remote_path) {
      dev.remote_path = tempRemotePath.trim();
      await api.saveDevice(dev);
      await ws.loadDevices();
      ws.showToast('远程配置文件路径已更新 ✓', 'success');
    }
    editingPathDevId = '';
  }
</script>

<div class="flex flex-1 flex-col min-h-0 overflow-hidden">
  <!-- 次级指标与基准对齐栏：拉平 4 个工作区水平高度，解决视觉突兀 -->
  <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 bg-white px-6 dark:border-zinc-800 dark:bg-zinc-950">
    <div class="flex items-center gap-4 text-xs">
      <span class="text-zinc-500 font-medium">节点集群状态:</span>
      <span class="flex items-center gap-1.5 font-mono text-[11px] text-zinc-600 dark:text-zinc-400">
        <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
        <span>在线: {onlineCount}</span>
      </span>
      <span class="flex items-center gap-1.5 font-mono text-[11px] text-zinc-600 dark:text-zinc-400">
        <span class="h-1.5 w-1.5 rounded-full bg-rose-500"></span>
        <span>阻断: {blockedCount}</span>
      </span>
      <span class="flex items-center gap-1.5 font-mono text-[11px] text-zinc-600 dark:text-zinc-400">
        <span class="h-1.5 w-1.5 rounded-full bg-blue-500"></span>
        <span>推送就绪: {pushReadyCount}</span>
      </span>
    </div>

    <div class="text-[11px] font-mono text-zinc-400">
      共接入 {ws.devices.length} 个实例
    </div>
  </div>

  <!-- 卡片主网格 -->
  <div class="flex-1 overflow-y-auto p-6 max-w-6xl mx-auto w-full space-y-4">
    {#if ws.devices.length === 0}
      <div class="flex h-80 flex-col items-center justify-center rounded-2xl border border-dashed border-zinc-300 p-8 text-center dark:border-zinc-800">
        <span class="text-xs text-zinc-400">当前暂无网络节点，点击右上角「接入新节点」进行配置</span>
      </div>
    {:else}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        {#each ws.devices as dev (dev.id)}
          <div class="rounded-2xl border border-zinc-200 bg-white p-5 shadow-xs hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-700 transition flex flex-col justify-between space-y-4">
            <div class="space-y-3">
              <div class="flex items-start justify-between">
                <div class="flex items-start gap-2.5">
                  {#if isDeviceManageMode}
                    <input
                      type="checkbox"
                      checked={selectedDeviceIds.includes(dev.id)}
                      onchange={(e) => {
                        if ((e.target as HTMLInputElement).checked) selectedDeviceIds = [...selectedDeviceIds, dev.id];
                        else selectedDeviceIds = selectedDeviceIds.filter((id: string) => id !== dev.id);
                      }}
                      class="rounded mt-1 cursor-pointer"
                    />
                  {/if}
                  <div>
                    <div class="flex items-center gap-2">
                      <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100">{dev.name}</h3>
                      <span class="rounded bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.2 font-mono text-[10px] text-zinc-400">{dev.id}</span>
                    </div>

                    {#if dev.tags && dev.tags.length > 0}
                      <div class="flex flex-wrap gap-1 mt-1.5">
                        {#each dev.tags as tag (tag)}
                          <span class="rounded bg-zinc-100 px-1.5 py-0.2 text-[10px] text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400 font-mono">#{tag}</span>
                        {/each}
                      </div>
                    {/if}
                  </div>
                </div>

                <!-- 状态与控制区 -->
                <div class="flex items-center gap-2">
                  {#if dev.enable_ping}
                    {#if ws.nodePingStatuses[dev.id]?.loading}
                      <span class="font-mono text-xs text-zinc-400 animate-pulse flex items-center gap-1">
                        <Icon name="refresh" size={12} class="animate-spin" />
                        <span>探测中</span>
                      </span>
                    {:else if ws.nodePingStatuses[dev.id]?.online}
                      {@const ms = ws.nodePingStatuses[dev.id]?.latency ?? 0}
                      <div class="flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-mono text-xs font-semibold {ms < 100 ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400' : ms < 250 ? 'bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400' : 'bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400'}">
                        <span class="h-1.5 w-1.5 rounded-full {ms < 100 ? 'bg-emerald-500 shadow-[0_0_6px_#10b981]' : ms < 250 ? 'bg-amber-500 shadow-[0_0_6px_#f59e0b]' : 'bg-rose-500 shadow-[0_0_6px_#f43f5e]'}"></span>
                        <span>{ms}ms</span>
                      </div>
                    {:else if ws.nodePingStatuses[dev.id]?.online === false}
                      <span title={ws.nodePingStatuses[dev.id]?.error} class="flex items-center gap-1 text-xs font-mono text-rose-500 font-semibold">
                        <span class="h-1.5 w-1.5 rounded-full bg-rose-500"></span>
                        <span>阻断</span>
                      </span>
                    {:else}
                      <span class="font-mono text-[11px] text-zinc-400">未探测</span>
                    {/if}
                  {:else}
                    <span class="font-mono text-[10px] text-zinc-400 bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 rounded">探活已禁</span>
                  {/if}

                  <div class="flex items-center border-l border-zinc-200 pl-2 dark:border-zinc-800 gap-1 text-zinc-400">
                    {#if dev.enable_ping}
                      <button title="立即探测此节点" onclick={() => ws.pingNode(dev.id)} class="p-1 hover:text-blue-500 cursor-pointer">
                        <Icon name="refresh" size={13} />
                      </button>
                    {/if}
                    <button title="克隆节点" onclick={() => onCloneDevice(dev)} class="p-1 hover:text-blue-500 cursor-pointer">
                      <Icon name="copy" size={13} />
                    </button>
                    <button title="编辑元数据" onclick={() => onEditDevice(dev)} class="p-1 hover:text-blue-500 cursor-pointer">
                      <Icon name="pencil" size={13} />
                    </button>
                    <button title="删除节点" onclick={() => triggerConfirm('移除网络节点？', `将从系统中移除 [${dev.name}]。`, () => ws.batchDeleteDevices([dev.id]))} class="p-1 hover:text-rose-500 cursor-pointer">
                      <Icon name="trash" size={13} />
                    </button>
                  </div>
                </div>
              </div>

              <!-- 面板详情 -->
              <div class="rounded-xl bg-zinc-50/70 p-3 dark:bg-zinc-950/60 border border-zinc-100 dark:border-zinc-800/80 space-y-2 text-xs">
                <div class="flex items-start justify-between">
                  <span class="text-zinc-400 font-mono text-[11px]">通信端点:</span>
                  <div class="space-y-0.5 text-right font-mono text-[11px]">
                    {#each dev.endpoints as ep (ep)}
                      <div class="text-zinc-700 dark:text-zinc-300">{ep}</div>
                    {:else}
                      <span class="text-zinc-400">未配置</span>
                    {/each}
                  </div>
                </div>

                <div class="flex items-center justify-between border-t border-zinc-200/50 pt-2 dark:border-zinc-800/50">
                  <span class="text-zinc-400 text-[11px]">生效方案:</span>
                  {#if dev.target_profile_id}
                    <button
                      onclick={() => {
                        ws.switchWorkspace('studio');
                        ws.selectProfile(dev.target_profile_id);
                      }}
                      class="rounded bg-blue-50 px-2 py-0.5 text-[11px] font-semibold text-blue-600 hover:bg-blue-100 dark:bg-blue-950/40 dark:text-blue-400 cursor-pointer transition flex items-center gap-1"
                    >
                      <Icon name="layers" size={11} />
                      <span>{ws.profiles.find(p => p.id === dev.target_profile_id)?.name || dev.target_profile_id}</span>
                    </button>
                  {:else}
                    <span class="text-zinc-400 text-[11px]">未指派</span>
                  {/if}
                </div>

                {#if dev.ssh_host || dev.endpoints.length > 0}
                  <div class="flex items-center justify-between border-t border-zinc-200/50 pt-2 dark:border-zinc-800/50 font-mono text-[11px]">
                    <span class="text-zinc-400">SSH 运维:</span>
                    <span class="text-zinc-600 dark:text-zinc-400 truncate max-w-55">
                      {dev.ssh_user}@{dev.ssh_host || dev.endpoints[0]}:{dev.ssh_port}
                    </span>
                  </div>
                {/if}
              </div>
            </div>

            <!-- 卡片底部：文件路径与推送控制 -->
            <div class="flex items-center justify-between pt-1 border-t border-zinc-100 dark:border-zinc-800/60">
              <div class="flex items-center gap-1.5 flex-1 min-w-0 mr-2">
                {#if editingPathDevId === dev.id}
                  <input
                    bind:value={tempRemotePath}
                    onblur={() => saveRemotePath(dev)}
                    onkeydown={(e) => { if (e.key === 'Enter') saveRemotePath(dev); }}
                    class="rounded border border-blue-500 bg-white px-2 py-0.5 text-[10px] font-mono w-full dark:bg-zinc-950 focus:outline-none"
                  />
                {:else}
                  <button
                    onclick={() => { editingPathDevId = dev.id; tempRemotePath = dev.remote_path; }}
                    title="点击直接修改远程配置文件路径"
                    class="text-[10px] text-zinc-400 hover:text-zinc-700 dark:hover:text-zinc-200 font-mono truncate text-left cursor-pointer flex items-center gap-1"
                  >
                    <span class="truncate">{dev.remote_path || '~/.config/sing-box/config.json'}</span>
                    <Icon name="pencil" size={10} class="opacity-50" />
                  </button>
                {/if}
              </div>

              {#if dev.enable_push && dev.target_profile_id}
                <button
                  disabled={isDeployingId === dev.id}
                  onclick={() => handleDeploy(dev)}
                  class="rounded-md bg-blue-600 px-3 py-1 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-2xs transition flex items-center gap-1.5 shrink-0"
                >
                  <Icon name="terminal" size={12} class={isDeployingId === dev.id ? 'animate-spin' : ''} />
                  <span>{isDeployingId === dev.id ? '推送中...' : '推送到节点'}</span>
                </button>
              {:else}
                <span class="text-[10px] text-zinc-400 font-mono">
                  {!dev.target_profile_id ? '未配方案' : '推送已禁用'}
                </span>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>