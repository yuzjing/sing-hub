<!-- frontend/src/lib/components/workspaces/FleetWorkspace.svelte -->
<script lang="ts">
  import { SvelteSet } from 'svelte/reactivity';
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Device } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';
  import TagFilter from '$lib/components/TagFilter.svelte';
  import DeployModal from '$lib/components/DeployModal.svelte';

  interface Props {
    isDeviceManageMode?: boolean;
    selectedDeviceIds?: string[];
    onEditDevice?: (dev: Device) => void;
    onCloneDevice?: (dev: Device) => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }

  let {
    isDeviceManageMode = $bindable(false),
    selectedDeviceIds = $bindable([]),
    onEditDevice = () => {},
    onCloneDevice = () => {},
    triggerConfirm = () => {}
  }: Props = $props();

  let isDeployingId = $state<string>('');
  let editingPathDevId = $state<string>('');
  let tempRemotePath = $state('');
  let expandedPropDevIds = $state<string[]>([]);
  let selectedFilterTags = $state<string[]>([]);

  let deployLogs = $state('');
  let deployStatus = $state<'running'|'success'|'error'>('running');
  let showDeployModal = $state(false);

  let availableTags = $derived.by(() => {
    const tags = new SvelteSet<string>();
    ws.devices.forEach((d: Device) => {
      (d.tags || []).forEach((t: string) => tags.add(t));
    });
    return Array.from(tags);
  });

  let filteredDevices = $derived.by(() => {
    return ws.devices.filter((dev: Device) => 
      selectedFilterTags.length === 0 || selectedFilterTags.some((t: string) => (dev.tags || []).includes(t))
    );
  });

  function toggleExpandProps(devId: string) {
    if (expandedPropDevIds.includes(devId)) {
      expandedPropDevIds = expandedPropDevIds.filter((id: string) => id !== devId);
    } else {
      expandedPropDevIds = [...expandedPropDevIds, devId];
    }
  }

  let onlineCount = $derived.by(() => ws.devices.filter((d: Device) => ws.nodePingStatuses[d.id]?.online === true).length);
  let blockedCount = $derived.by(() => ws.devices.filter((d: Device) => ws.nodePingStatuses[d.id]?.online === false).length);
  let pushReadyCount = $derived.by(() => ws.devices.filter((d: Device) => d.enable_push && !!d.target_profile_id).length);

  async function handleDeploy(dev: Device) {
    if (!dev.enable_push || !dev.target_profile_id) return;
    isDeployingId = dev.id;
    deployLogs = '';
    deployStatus = 'running';
    showDeployModal = true;
    try {
      const res = await api.deployDevice(dev.id);
      deployLogs = res.logs + `\n\n[✔] 推送与重载成功\n[i] 远端内核: ${res.remote_version}\n[i] 最终哈希: ${res.verified_hash}`;
      deployStatus = 'success';
      ws.showToast('推送部署成功 ✓', 'success');
    } catch (e: unknown) {
      const err = e as { message?: string; logs?: string };
      deployLogs = (err.logs || '') + `\n\n[✖] 部署过程中断: ${err.message || '未知错误'}`;
      deployStatus = 'error';
      ws.showToast(err.message || '推送失败', 'error');
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

  function focusEl(node: HTMLInputElement) {
    node.focus();
  }
</script>

<div class="flex flex-1 flex-col min-h-0 overflow-hidden">
  <!-- 指标与控制头 -->
  <div class="flex h-14 shrink-0 items-center justify-between border-b border-zinc-200 bg-white px-6 dark:border-zinc-800 dark:bg-zinc-950">
    <div class="flex items-center gap-5 text-sm">
      <span class="text-zinc-500 font-bold tracking-wide">集群指标:</span>
      <span class="flex items-center gap-1.5 font-mono text-xs text-zinc-700 dark:text-zinc-300">
        <span class="h-2.5 w-2.5 rounded-full bg-emerald-500 shadow-2xs shadow-emerald-500/50"></span>
        <span class="font-bold">在线: {onlineCount}</span>
      </span>
      <span class="flex items-center gap-1.5 font-mono text-xs text-zinc-700 dark:text-zinc-300">
        <span class="h-2.5 w-2.5 rounded-full bg-rose-500 shadow-2xs shadow-rose-500/50"></span>
        <span class="font-bold">阻断: {blockedCount}</span>
      </span>
      <span class="flex items-center gap-1.5 font-mono text-xs text-zinc-700 dark:text-zinc-300">
        <span class="h-2.5 w-2.5 rounded-full bg-blue-500 shadow-2xs shadow-blue-500/50"></span>
        <span class="font-bold">推送就绪: {pushReadyCount}</span>
      </span>
    </div>

    <div class="flex items-center gap-4">
      <div class="w-64">
        <TagFilter {availableTags} bind:selectedTags={selectedFilterTags} borderless={true} />
      </div>
      <div class="text-xs font-mono text-zinc-500 font-semibold border-l border-zinc-200 dark:border-zinc-800 pl-4">
        显示: {filteredDevices.length} / {ws.devices.length} 台
      </div>
    </div>
  </div>

  <div class="flex-1 overflow-y-auto p-6 max-w-7xl mx-auto w-full space-y-5">
    {#if filteredDevices.length === 0}
      <div class="flex h-96 flex-col items-center justify-center rounded-3xl border border-dashed border-zinc-300 p-8 text-center dark:border-zinc-800 bg-white/50 dark:bg-zinc-950/50">
        <span class="text-sm font-medium text-zinc-500 dark:text-zinc-400">
          {ws.devices.length === 0 ? '当前暂无网络节点，点击右上角「接入新节点」进行配置' : '没有匹配标签的节点'}
        </span>
      </div>
    {:else}
      <div class="grid grid-cols-1 xl:grid-cols-2 gap-5">
        {#each filteredDevices as dev (dev.id)}
          <!-- Svelte 5 合规：@const 必须是 #each 的直接子节点 -->
          {@const targetProfile = ws.profiles.find(p => p.id === dev.target_profile_id)}
          {@const allProps = dev.props && dev.props.length > 0 ? dev.props : dev.endpoints.map((ep: string, i: number) => ({ key: 'ep_' + i, label: '端点' + (i+1), value: ep }))}
          {@const isExpanded = expandedPropDevIds.includes(dev.id)}
          {@const visibleProps = isExpanded ? allProps : allProps.slice(0, 3)}

          <div class="rounded-3xl border border-zinc-200 bg-white p-6 shadow-sm hover:shadow-md dark:border-zinc-800 dark:bg-zinc-900 transition-all flex flex-col justify-between space-y-4">
            <div class="space-y-4">
              <div class="flex items-start justify-between">
                <div class="flex items-start gap-3">
                  {#if isDeviceManageMode}
                    <input
                      type="checkbox"
                      checked={selectedDeviceIds.includes(dev.id)}
                      onchange={(e) => {
                        if ((e.target as HTMLInputElement).checked) selectedDeviceIds = [...selectedDeviceIds, dev.id];
                        else selectedDeviceIds = selectedDeviceIds.filter((id: string) => id !== dev.id);
                      }}
                      class="rounded h-4 w-4 mt-0.5 cursor-pointer"
                    />
                  {/if}
                  <div>
                    <!-- 节点版本展示点 1：标题栏双版本对比 -->
                    <div class="flex items-center gap-2.5 flex-wrap">
                      <h3 class="text-base font-extrabold text-zinc-900 dark:text-zinc-100">{dev.name}</h3>
                      <span class="rounded-md bg-zinc-100 dark:bg-zinc-800 px-2 py-0.5 font-mono text-xs font-semibold text-zinc-500 dark:text-zinc-400 border border-zinc-200 dark:border-zinc-700">{dev.id}</span>
                      
                      {#if dev.last_remote_ver}
                        <span class="rounded-md bg-blue-50 text-blue-600 border border-blue-200 dark:bg-blue-950/40 dark:text-blue-400 dark:border-blue-900/50 px-2 py-0.5 font-mono text-[10px] font-bold" title="远程实体物理内核">
                          内核: {dev.last_remote_ver}
                        </span>
                      {/if}

                      {#if targetProfile?.sing_box_version && targetProfile.sing_box_version !== 'universal'}
                        <span class="rounded-md bg-emerald-50 text-emerald-600 border border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-900/50 px-2 py-0.5 font-mono text-[10px] font-bold" title="指派方案要求内核">
                          目标: v{targetProfile.sing_box_version}
                        </span>
                      {/if}
                    </div>

                    {#if dev.tags && dev.tags.length > 0}
                      <div class="flex flex-wrap gap-1.5 mt-2.5">
                        {#each dev.tags as tag (tag)}
                          <span class="rounded-lg bg-blue-50 dark:bg-blue-950/40 px-2 py-0.5 text-[11px] font-bold text-blue-600 dark:text-blue-400 font-mono border border-blue-100 dark:border-blue-900/50">#{tag}</span>
                        {/each}
                      </div>
                    {/if}
                  </div>
                </div>

                <div class="flex items-center gap-3">
                  {#if dev.enable_ping}
                    {#if ws.nodePingStatuses[dev.id]?.loading}
                      <span class="font-mono text-sm font-semibold text-zinc-500 animate-pulse flex items-center gap-1.5">
                        <Icon name="refresh" size={14} class="animate-spin" />
                        <span>探测中...</span>
                      </span>
                    {:else if ws.nodePingStatuses[dev.id]?.online}
                      {@const ms = ws.nodePingStatuses[dev.id]?.latency ?? 0}
                      <div class="flex items-center gap-2 rounded-full px-3 py-1 font-mono text-sm font-bold border {ms < 100 ? 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/60 dark:text-emerald-400 dark:border-emerald-900/50' : ms < 250 ? 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/60 dark:text-amber-400 dark:border-amber-900/50' : 'bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-950/60 dark:text-rose-400 dark:border-rose-900/50'} shadow-2xs">
                        <span class="h-2 w-2 rounded-full {ms < 100 ? 'bg-emerald-500 shadow-[0_0_6px_#10b981]' : ms < 250 ? 'bg-amber-500 shadow-[0_0_6px_#f59e0b]' : 'bg-rose-500 shadow-[0_0_6px_#f43f5e]'}"></span>
                        <span>{ms}ms</span>
                      </div>
                    {:else if ws.nodePingStatuses[dev.id]?.online === false}
                      <span title={ws.nodePingStatuses[dev.id]?.error} class="flex items-center gap-1.5 text-sm font-mono text-rose-600 font-extrabold bg-rose-50 border border-rose-200 dark:bg-rose-950/60 dark:border-rose-900/50 px-3 py-1 rounded-full shadow-2xs">
                        <span class="h-2 w-2 rounded-full bg-rose-500"></span>
                        <span>阻断</span>
                      </span>
                    {:else}
                      <span class="font-mono text-xs font-semibold text-zinc-400 bg-zinc-50 dark:bg-zinc-800/50 px-3 py-1 rounded-full border border-zinc-200 dark:border-zinc-800">等待探测</span>
                    {/if}
                  {:else}
                    <span class="font-mono text-xs font-semibold text-zinc-400 bg-zinc-100 dark:bg-zinc-800 px-2 py-1 rounded-lg">探活已禁</span>
                  {/if}

                  <div class="flex items-center border-l border-zinc-200 pl-3 dark:border-zinc-800 gap-1.5 text-zinc-400">
                    {#if dev.enable_ping}
                      <button title="探测节点" onclick={() => ws.pingNode(dev.id)} type="button" class="p-1.5 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 hover:text-blue-600 transition cursor-pointer">
                        <Icon name="refresh" size={15} />
                      </button>
                    {/if}
                    <button title="克隆节点" onclick={() => onCloneDevice(dev)} type="button" class="p-1.5 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 hover:text-blue-600 transition cursor-pointer">
                      <Icon name="copy" size={15} />
                    </button>
                    <button title="编辑节点" onclick={() => onEditDevice(dev)} type="button" class="p-1.5 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 hover:text-blue-600 transition cursor-pointer">
                      <Icon name="pencil" size={15} />
                    </button>
                    <button title="删除节点" onclick={() => triggerConfirm('移除网络节点？', '将从系统中移除 [' + dev.name + ']。', () => ws.batchDeleteDevices([dev.id]))} type="button" class="p-1.5 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 hover:text-rose-600 transition cursor-pointer">
                      <Icon name="trash" size={15} />
                    </button>
                  </div>
                </div>
              </div>

              <!-- 卡片主体：运行环境、属性与目标 -->
              <div class="rounded-2xl bg-zinc-50/70 p-4 dark:bg-zinc-950/50 border border-zinc-200/60 dark:border-zinc-800/80 space-y-4">
                
                <div class="flex items-center justify-between border-b border-zinc-200/60 dark:border-zinc-800/80 pb-2.5">
                  <div class="flex items-center gap-2">
                    <span class="px-2 py-0.5 rounded-md bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300 font-bold text-[10px] font-mono border border-blue-200 dark:border-blue-800/50 uppercase">
                      {dev.deploy_strategy || 'systemd'}
                    </span>
                    <span class="font-mono text-xs font-semibold text-zinc-600 dark:text-zinc-400">
                      {dev.service_name || 'sing-box'}
                    </span>
                  </div>
                  {#if dev.deploy_strategy === 'standalone'}
                    <span class="text-[10px] text-amber-600 font-semibold bg-amber-50 border border-amber-200 px-2 py-0.5 rounded-md dark:bg-amber-950/40 dark:border-amber-900/50">仅下发不重启</span>
                  {/if}
                </div>

                <div>
                  <div class="flex items-center justify-between mb-2">
                    <span class="text-zinc-500 font-bold text-xs uppercase tracking-wider">端点与网络属性 ({allProps.length})</span>
                    {#if allProps.length > 3}
                      <button
                        onclick={() => toggleExpandProps(dev.id)}
                        type="button"
                        class="text-xs font-mono text-blue-600 dark:text-blue-400 font-bold hover:underline cursor-pointer bg-blue-50 dark:bg-blue-950/40 px-2 py-0.5 rounded-full"
                      >
                        {isExpanded ? '收起 ▴' : `查看全部 (${allProps.length}) ▾`}
                      </button>
                    {/if}
                  </div>

                  <div class="space-y-1.5">
                    {#each visibleProps as p (p.key)}
                      <div class="flex items-center justify-between font-mono text-xs bg-white dark:bg-zinc-900 px-3 py-1.5 rounded-lg border border-zinc-200 dark:border-zinc-800 shadow-2xs hover:border-blue-300 transition">
                        <span class="text-zinc-500 font-medium truncate max-w-[40%]">{p.label || p.key}:</span>
                        <span class="text-zinc-900 dark:text-zinc-100 font-bold truncate max-w-[55%]">{p.value}</span>
                      </div>
                    {:else}
                      <span class="text-zinc-400 text-xs italic block p-2">未配置任何通信端点</span>
                    {/each}
                  </div>
                </div>

                <div class="grid grid-cols-2 gap-4 border-t border-zinc-200/60 pt-4 dark:border-zinc-800/80">
                  <!-- 节点版本展示点 2：生效方案插槽内带版本 Badge -->
                  <div class="space-y-1.5">
                    <span class="text-zinc-500 font-bold text-xs uppercase tracking-wider block">生效方案绑定</span>
                    {#if dev.target_profile_id}
                      <button
                        onclick={() => {
                          ws.switchWorkspace('studio');
                          ws.selectProfile(dev.target_profile_id);
                        }}
                        type="button"
                        class="w-full text-left rounded-lg bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 px-3 py-1.5 text-xs font-bold text-amber-700 dark:text-amber-400 hover:border-amber-400 transition flex items-center justify-between shadow-2xs cursor-pointer"
                      >
                        <span class="truncate">{targetProfile?.name || dev.target_profile_id}</span>
                        <div class="flex items-center gap-1.5 shrink-0">
                          {#if targetProfile?.sing_box_version && targetProfile.sing_box_version !== 'universal'}
                            <span class="px-1.5 py-0.2 rounded bg-emerald-50 text-emerald-600 border border-emerald-200 dark:bg-emerald-950/60 dark:text-emerald-400 text-[9px] font-mono">
                              v{targetProfile.sing_box_version}
                            </span>
                          {/if}
                          <Icon name="layers" size={13} />
                        </div>
                      </button>
                    {:else}
                      <span class="text-zinc-400 text-xs font-medium bg-zinc-100 dark:bg-zinc-900 px-3 py-1.5 rounded-lg border border-dashed border-zinc-300 dark:border-zinc-700 block">未指派方案</span>
                    {/if}
                  </div>

                  <div class="space-y-1.5">
                    <span class="text-zinc-500 font-bold text-xs uppercase tracking-wider block">SSH 部署凭据</span>
                    {#if dev.ssh_host || dev.endpoints.length > 0}
                      <div class="w-full rounded-lg bg-white dark:bg-zinc-900 border border-zinc-200 dark:border-zinc-800 px-3 py-1.5 text-xs font-mono font-bold text-zinc-800 dark:text-zinc-300 shadow-2xs truncate" title="{dev.ssh_user}@{dev.ssh_host || dev.endpoints[0]}:{dev.ssh_port}">
                        {dev.ssh_user}@{dev.ssh_host || dev.endpoints[0]}:{dev.ssh_port}
                      </div>
                    {:else}
                      <span class="text-zinc-400 text-xs font-medium bg-zinc-100 dark:bg-zinc-900 px-3 py-1.5 rounded-lg border border-dashed border-zinc-300 dark:border-zinc-700 block">未配置 SSH</span>
                    {/if}
                  </div>
                </div>
              </div>
            </div>

            <!-- 卡片底部极客运维栏 -->
            <!-- 找到卡片底部极客运维栏：将原本只有终端路径的区域，更新为在红框位置插入内核状态胶囊 -->
            <div class="flex items-center justify-between pt-3 border-t border-zinc-100 dark:border-zinc-800/80 shrink-0">
              <div class="flex items-center gap-2 flex-1 min-w-0 mr-4">
                <Icon name="terminal" size={14} class="text-zinc-400 shrink-0" />
                {#if editingPathDevId === dev.id}
                  <input
                    bind:value={tempRemotePath}
                    use:focusEl
                    onblur={() => saveRemotePath(dev)}
                    onkeydown={(e) => { if (e.key === 'Enter') saveRemotePath(dev); }}
                    class="rounded-md border-2 border-blue-500 bg-white px-2.5 py-1 text-xs font-mono w-full dark:bg-zinc-950 focus:outline-none shadow-sm transition"
                  />
                {:else}
                  <button
                    onclick={() => { editingPathDevId = dev.id; tempRemotePath = dev.remote_path; }}
                    type="button"
                    title="点击修改配置文件下发路径"
                    class="text-xs text-zinc-500 hover:text-blue-600 dark:hover:text-blue-400 font-mono font-medium truncate text-left cursor-pointer flex items-center gap-1.5 transition"
                  >
                    <span class="truncate">{dev.remote_path || '~/.config/sing-box/config.json'}</span>
                    <Icon name="pencil" size={11} class="opacity-50" />
                  </button>
                {/if}
              </div>

              <div class="flex items-center gap-3">
                <!-- 核心新增：红框位置节点内核版本胶囊 -->
                {#if dev.last_remote_ver}
                  <div class="inline-flex items-center rounded-lg border border-zinc-200/80 bg-zinc-100/70 p-0.5 dark:border-zinc-700/80 dark:bg-zinc-800/60 shadow-2xs font-mono text-xs">
                    <span class="px-2 py-0.5 rounded-md bg-blue-500/10 text-blue-600 dark:text-blue-400 font-bold text-[11px]" title="节点实际内核版本">
                      {dev.last_remote_ver}
                    </span>
                    {#if targetProfile?.sing_box_version && targetProfile.sing_box_version !== 'universal'}
                      {@const isMatch = dev.last_remote_ver.includes(targetProfile.sing_box_version)}
                      <span class="px-1.5 py-0.5 font-bold text-[10px] {isMatch ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}" title={isMatch ? '内核版本与生效方案高度契合' : '内核版本与方案版本存在差异'}>
                        {isMatch ? '✓ 匹配' : '⚠️ 差异'}
                      </span>
                    {/if}
                  </div>
                {:else}
                  <span class="text-xs font-mono text-zinc-400 italic">待部署探测版本</span>
                {/if}

                <!-- 部署按钮 -->
                {#if dev.enable_push && dev.target_profile_id}
                  <button
                    disabled={isDeployingId === dev.id}
                    onclick={() => handleDeploy(dev)}
                    type="button"
                    class="rounded-xl bg-blue-600 px-4 py-1.5 text-sm font-bold text-white hover:bg-blue-500 cursor-pointer shadow-md shadow-blue-600/20 transition flex items-center gap-2 shrink-0 disabled:opacity-60"
                  >
                    <Icon name="terminal" size={14} class={isDeployingId === dev.id ? 'animate-spin' : ''} />
                    <span>{isDeployingId === dev.id ? '部署预检中...' : '安全部署配置'}</span>
                  </button>
                {:else}
                  <span class="text-xs font-bold text-zinc-400 bg-zinc-100 dark:bg-zinc-800 px-3 py-1.5 rounded-xl border border-zinc-200 dark:border-zinc-700 shadow-2xs">
                    {!dev.target_profile_id ? '需先指派方案' : '推送已禁用'}
                  </span>
                {/if}
              </div>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<DeployModal
  open={showDeployModal}
  deviceName={ws.devices.find((d: Device) => d.id === isDeployingId)?.name || ''}
  logs={deployLogs}
  status={deployStatus}
  onClose={() => showDeployModal = false}
/>