<!-- frontend/src/lib/components/workspaces/StudioWorkspace.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Profile, Module, Component, Device } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ParamBinder from '$lib/components/ParamBinder.svelte';
  import TagFilter from '$lib/components/TagFilter.svelte';

  interface Props {
    isProfileManageMode?: boolean;
    selectedProfileIds?: string[];
    onEditProfile?: (p: Profile) => void;
    onCloneProfile?: (p: Profile) => void;
    onNewProfile?: (e?: MouseEvent) => void;
    onOpenMount?: (e?: MouseEvent) => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }

  let {
    isProfileManageMode = $bindable(false),
    selectedProfileIds = $bindable([]),
    onEditProfile = () => {},
    onCloneProfile = () => {},
    onNewProfile = () => {},
    onOpenMount = () => {},
    triggerConfirm = () => {}
  }: Props = $props();

  let copiedJSON = $state(false);
  let copiedSub = $state(false);

  let activeLinterTab = $state<'stable' | 'alpha'>('stable');
  let showDetailModal = $state(false);

  let isDualTrack = $derived.by(() => {
    return !!(ws.validationReport?.stable && ws.validationReport?.alpha);
  });

  let currentTabResult = $derived.by(() => {
    const r = ws.validationReport;
    if (!r || !r.enabled) return null;
    if (r.stable && !r.alpha) return r.stable;
    if (r.alpha && !r.stable) return r.alpha;
    return activeLinterTab === 'stable' ? r.stable : r.alpha;
  });

  // 方案标签统计与多选
  let selectedFilterTags = $state<string[]>([]);
  let tagCounts = $derived.by(() => {
    const counts: Record<string, number> = {};
    ws.profiles.forEach((p: Profile) => {
      (p.tags || []).forEach((t: string) => {
        counts[t] = (counts[t] || 0) + 1;
      });
    });
    return counts;
  });
  let availableTags = $derived.by(() => Object.keys(tagCounts));

  let expandedModuleId = $state<string | null>(null);
  let inlineDraftParams = $state<Record<string, string | number | boolean>>({});
  let isInlineDirty = $state(false);

  function safeParse(raw: string | undefined): Record<string, string | number | boolean> {
    if (!raw) return {};
    try {
      return JSON.parse(raw);
    } catch {
      return {};
    }
  }

  function detectTemplateFieldMap(code: string): Record<string, string> {
    const mapping: Record<string, string> = {};
    if (!code) return mapping;
    const regex = /"([^"\\]*(?:\\.[^"\\]*)*)"\s*:\s*"?\{\{\s*\.([a-zA-Z0-9_.]+)/g;
    for (const match of code.matchAll(regex)) {
      const fieldName = match[1];
      const varName = match[2];
      mapping[varName] = fieldName;
      mapping['.' + varName] = fieldName;
    }
    return mapping;
  }

  function toggleInlineExpand(mod: Module) {
    if (expandedModuleId === mod.id) {
      expandedModuleId = null;
      isInlineDirty = false;
    } else {
      expandedModuleId = mod.id;
      const latestMod = ws.modules.find((m: Module) => m.id === mod.id) || mod;
      inlineDraftParams = safeParse(latestMod.params_json);
      isInlineDirty = false;
    }
  }

  function getTemplateVars(tmpl: Component | null): string[] {
    if (!tmpl || !tmpl.payload_tmpl) return [];
    const matches = tmpl.payload_tmpl.matchAll(/\{\{\s*\.([a-zA-Z0-9_.]+)(?:\s*\|[^}]+)?\s*\}\}/g);
    const unique: string[] = [];
    for (const m of matches) {
      if (!unique.includes(m[1])) unique.push(m[1]);
    }
    return unique;
  }

  let referencedNodes = $derived.by(() => {
    if (!ws.activeProfile) return [];
    const nodeIds = new Set<string>();
    for (const item of ws.resolvedModuleBindings) {
      const p = safeParse(item.module.params_json);
      for (const val of Object.values(p)) {
        const s = String(val);
        if (s.startsWith('@node:')) {
          const parts = s.replace('@node:', '').split('.');
          if (parts.length > 0) nodeIds.add(parts[0]);
        }
      }
    }
    return ws.devices.filter((d: Device) => nodeIds.has(d.id));
  });

  let targetDevices = $derived.by(() => {
    if (!ws.activeProfile) return [];
    return ws.devices.filter((d: Device) => d.target_profile_id === ws.activeProfile?.id);
  });

  async function saveInlineParams(mod: Module) {
    const latestMod = ws.modules.find((m: Module) => m.id === mod.id) || mod;
    latestMod.params_json = JSON.stringify(inlineDraftParams);
    await api.saveModule(latestMod);
    await ws.loadModules();
    await ws.loadProfiles();
    isInlineDirty = false;
    ws.showToast('积木 [' + mod.name + '] 实参已更新并重新编译 ✓', 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }

  function cancelInlineParams(mod: Module) {
    const latestMod = ws.modules.find((m: Module) => m.id === mod.id) || mod;
    inlineDraftParams = safeParse(latestMod.params_json);
    isInlineDirty = false;
    ws.showToast('已还原实参', 'info');
  }

  async function copySubURL(token: string) {
    const url = window.location.origin + '/sub/' + token;
    await navigator.clipboard.writeText(url);
    copiedSub = true;
    ws.showToast('订阅链接已复制 ✓', 'success');
    setTimeout(() => (copiedSub = false), 1400);
  }

  async function copyCompiledJSON() {
    if (!ws.compiledJSON) return;
    await navigator.clipboard.writeText(ws.compiledJSON);
    copiedJSON = true;
    ws.showToast('完整配置 JSON 已复制 ✓', 'success');
    setTimeout(() => (copiedJSON = false), 1400);
  }
</script>

<div class="flex flex-1 min-h-0 overflow-hidden relative">
  <!-- 左栏：方案配方列表 -->
  <aside class="flex w-72 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50/70 dark:border-zinc-800 dark:bg-zinc-950/40">
    <div class="flex h-14 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
      <div class="flex items-center gap-2">
        <Icon name="layers" size={16} color="amber" />
        <span class="text-sm font-bold uppercase tracking-wider text-zinc-600 dark:text-zinc-300">方案配方 ({ws.profiles.length})</span>
      </div>
      <div class="flex items-center gap-1.5">
        <button
          title={isProfileManageMode ? '退出管理' : '批量整理方案'}
          onclick={() => { isProfileManageMode = !isProfileManageMode; selectedProfileIds = []; }}
          type="button"
          class="flex h-7 w-7 items-center justify-center rounded-lg transition cursor-pointer shadow-2xs {isProfileManageMode ? 'bg-amber-100 text-amber-700 border border-amber-300 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-800' : 'bg-white border border-zinc-200 text-zinc-500 hover:bg-zinc-100 dark:bg-zinc-900 dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800'}"
        >
          <Icon name="sliders" size={14} />
        </button>
        <button
          title="新建方案配方"
          onclick={onNewProfile}
          type="button"
          class="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-50 border border-blue-200 text-blue-600 hover:bg-blue-100 shadow-2xs dark:bg-blue-950/50 dark:border-blue-900/50 dark:text-blue-400 cursor-pointer transition"
        >
          <Icon name="plus" size={15} />
        </button>
      </div>
    </div>

    <!-- 方案标签多选筛选 (带项目数量) -->
    <TagFilter {availableTags} {tagCounts} bind:selectedTags={selectedFilterTags} />

    <div class="flex-1 overflow-y-auto p-3 space-y-2">
      {#each ws.profiles.filter((p: Profile) => selectedFilterTags.length === 0 || selectedFilterTags.some((t: string) => (p.tags || []).includes(t))) as p (p.id)}
        {@const isActive = ws.activeProfileId === p.id}
        <div class="group relative flex items-center justify-between rounded-xl px-3.5 py-3 text-sm transition border shadow-2xs {isActive && !isProfileManageMode ? 'border-amber-400/60 bg-white text-zinc-900 font-extrabold dark:border-amber-500/50 dark:bg-zinc-900 dark:text-white' : 'border-zinc-200 bg-white/50 text-zinc-600 hover:bg-white dark:border-zinc-800 dark:bg-zinc-950/50 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
          {#if isActive && !isProfileManageMode}
            <span class="absolute left-0 top-2.5 bottom-2.5 w-1.5 rounded-r-full bg-amber-500 shadow-[0_0_8px_#f59e0b]"></span>
          {/if}

          <div class="flex items-center gap-2.5 truncate flex-1 pl-1.5">
            {#if isProfileManageMode}
              <input
                type="checkbox"
                checked={selectedProfileIds.includes(p.id)}
                onchange={(e) => {
                  if ((e.target as HTMLInputElement).checked) selectedProfileIds = [...selectedProfileIds, p.id];
                  else selectedProfileIds = selectedProfileIds.filter((id: string) => id !== p.id);
                }}
                class="rounded h-3.5 w-3.5 cursor-pointer"
              />
            {/if}
            <button onclick={() => ws.selectProfile(p.id)} type="button" class="truncate text-left flex-1 cursor-pointer">
              {p.name}
            </button>
          </div>

          <div class="flex items-center gap-1.5 shrink-0">
            {#if !isProfileManageMode}
              {#if p.tags && p.tags.length > 0}
                <span class="text-[10px] font-mono font-bold px-2 py-0.5 rounded-md bg-zinc-100 border border-zinc-200 dark:bg-zinc-800 dark:border-zinc-700 text-zinc-500">#{p.tags[0]}</span>
              {/if}
              {#if p.capabilities.enable_subscribe}
                <button onclick={() => copySubURL(p.token)} type="button" title="复制订阅链接" class="text-zinc-400 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition">
                  <Icon name="link" size={14} />
                </button>
              {/if}
            {:else}
              <button title="克隆方案" onclick={() => onCloneProfile(p)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                <Icon name="copy" size={14} />
              </button>
              <button title="编辑元属性" onclick={() => onEditProfile(p)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                <Icon name="pencil" size={14} />
              </button>
              <button title="删除方案" onclick={() => triggerConfirm('确认删除方案？', '即将永久删除方案 [' + p.name + ']。', () => ws.batchDeleteProfiles([p.id]))} type="button" class="text-zinc-400 hover:text-rose-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                <Icon name="trash" size={14} />
              </button>
            {/if}
          </div>
        </div>
      {:else}
        <div class="py-12 text-center text-sm text-zinc-400 italic">暂无匹配方案</div>
      {/each}
    </div>
  </aside>

  <!-- 中间流水线 -->
  <main class="flex flex-1 min-w-0 flex-col overflow-hidden border-r border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
    <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
      <div class="flex items-center gap-2">
        <span class="text-xs text-zinc-400">当前方案:</span>
        <strong class="text-sm text-zinc-900 dark:text-zinc-100">{ws.activeProfile?.name || '请选择方案'}</strong>
        {#if ws.activeProfile?.sing_box_version}
          <span class="rounded bg-zinc-100 dark:bg-zinc-800 px-2 py-0.5 font-mono text-[11px] font-medium text-zinc-500">{ws.activeProfile.sing_box_version}</span>
        {/if}
      </div>

      {#if ws.activeProfile}
        <div class="flex items-center gap-2.5">
          <button
            onclick={onOpenMount}
            type="button"
            class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 shadow-2xs hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer transition flex items-center gap-1.5"
          >
            <Icon name="plus" size={14} />
            <span>挑选装配</span>
          </button>
          <button
            onclick={() => ws.triggerCompile()}
            type="button"
            class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white shadow-2xs hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5"
          >
            <Icon name="check" size={14} />
            <span>{ws.isCompiling ? '编译中...' : '编译保存'}</span>
          </button>
        </div>
      {/if}
    </div>

    <div class="flex-1 overflow-y-auto p-5 space-y-4">
      <!-- 拓扑感知卡片 -->
      {#if ws.activeProfile}
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-4 dark:border-zinc-800 dark:bg-zinc-900/60 space-y-2">
            <span class="text-xs font-bold text-zinc-600 dark:text-zinc-300 flex items-center gap-1.5">
              <Icon name="server" size={14} color="blue" />
              <span>配置引用的节点集群 ({referencedNodes.length})</span>
            </span>
            <div class="flex flex-wrap gap-2 pt-1">
              {#each referencedNodes as dev (dev.id)}
                <span class="px-2.5 py-1 rounded bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300 text-xs font-mono font-medium shadow-2xs">
                  {dev.name}
                </span>
              {:else}
                <span class="text-xs text-zinc-400 italic">未引用任何节点属性</span>
              {/each}
            </div>
          </div>

          <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-4 dark:border-zinc-800 dark:bg-zinc-900/60 space-y-2">
            <span class="text-xs font-bold text-zinc-600 dark:text-zinc-300 flex items-center gap-1.5">
              <Icon name="terminal" size={14} color="emerald" />
              <span>下发推送目标实例 ({targetDevices.length})</span>
            </span>
            <div class="flex flex-wrap gap-2 pt-1">
              {#each targetDevices as dev (dev.id)}
                <span class="px-2.5 py-1 rounded bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300 text-xs font-mono font-medium shadow-2xs">
                  {dev.name} ({dev.ssh_user}@{dev.ssh_host || dev.endpoints[0]})
                </span>
              {:else}
                <span class="text-xs text-zinc-400 italic">尚未指派给任何节点</span>
              {/each}
            </div>
          </div>
        </div>
      {/if}

      <!-- 积木流水线 -->
      {#if ws.resolvedModuleBindings.length === 0}
        <div class="flex h-72 flex-col items-center justify-center rounded-xl border border-dashed border-zinc-300 p-8 text-center dark:border-zinc-800">
          <span class="text-sm text-zinc-400">当前方案暂未装配成品积木，点击右上角「挑选装配」添加</span>
        </div>
      {:else}
        {#each ws.resolvedModuleBindings as item, idx (item.binding.id ?? idx)}
          {@const latestMod = ws.modules.find((m: Module) => m.id === item.module.id) || item.module}
          {@const currentParams = safeParse(latestMod.params_json)}
          {@const tmplVars = getTemplateVars(item.template)}
          {@const fieldMap = detectTemplateFieldMap(item.template?.payload_tmpl || '')}
          {@const isExpanded = expandedModuleId === item.module.id}

          <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-5 shadow-xs hover:shadow-md hover:border-zinc-300 dark:border-zinc-800/90 dark:bg-zinc-900/60 dark:hover:border-zinc-700 transition-all space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <span class="font-mono text-sm font-bold text-zinc-400 dark:text-zinc-600">0{idx + 1}</span>
                <span class="text-base font-bold text-zinc-900 dark:text-zinc-100">{item.module.name}</span>
                <span class="rounded-md bg-zinc-200/80 px-2 py-0.5 font-mono text-xs font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                  {item.template?.mount_path || '自定义路径'}
                </span>
                <span class="text-xs text-zinc-500 font-mono">
                  母版: {item.template?.name || item.module.template_id}
                </span>
              </div>

              <div class="flex items-center gap-1.5">
                <button
                  onclick={() => toggleInlineExpand(item.module)}
                  type="button"
                  class="rounded-lg border border-zinc-200 bg-white px-3 py-1 text-xs text-zinc-600 hover:border-zinc-300 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 cursor-pointer shadow-2xs transition"
                >
                  {isExpanded ? '收起配置' : '调整实参'}
                </button>
                <div class="flex items-center gap-1 bg-zinc-100 dark:bg-zinc-800 p-0.5 rounded-lg border border-zinc-200 dark:border-zinc-700 ml-2">
                  <button disabled={idx === 0} onclick={() => ws.moveModuleBinding(idx, 'up')} type="button" class="rounded px-2 py-1 text-xs text-zinc-500 hover:bg-white disabled:opacity-30 dark:hover:bg-zinc-700 cursor-pointer transition">↑</button>
                  <button disabled={idx === ws.resolvedModuleBindings.length - 1} onclick={() => ws.moveModuleBinding(idx, 'down')} type="button" class="rounded px-2 py-1 text-xs text-zinc-500 hover:bg-white disabled:opacity-30 dark:hover:bg-zinc-700 cursor-pointer transition">↓</button>
                  <button onclick={() => ws.detachModule(item.binding.id ?? item.module.id)} type="button" class="rounded px-2 py-1 text-zinc-400 hover:bg-white hover:text-rose-500 dark:hover:bg-zinc-700 cursor-pointer transition">
                    <Icon name="close" size={13} />
                  </button>
                </div>
              </div>
            </div>

            <!-- 就地展开且内嵌 ParamBinder -->
            {#if isExpanded}
              <div class="pt-4 border-t border-zinc-200/70 dark:border-zinc-800/80 space-y-4 animate-in fade-in duration-100">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300">
                    实参赋值 (保存后右侧全局配置实时联动编译):
                  </span>
                  <div class="flex items-center gap-2">
                    {#if isInlineDirty}
                      <button
                        onclick={() => cancelInlineParams(item.module)}
                        type="button"
                        class="text-xs text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-200 cursor-pointer font-medium px-2"
                      >
                        放弃修改
                      </button>
                    {/if}
                    <button
                      onclick={() => saveInlineParams(item.module)}
                      type="button"
                      class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-sm transition"
                    >
                      应用实参
                    </button>
                  </div>
                </div>

                <ParamBinder
                  templateVars={tmplVars}
                  templateCode={item.template?.payload_tmpl || ''}
                  bind:params={inlineDraftParams}
                  onChange={() => {
                    isInlineDirty = true;
                  }}
                />
              </div>
            {:else}
              <!-- 收起状态只展示目标 JSON 字段名与值 -->
              <div class="pt-2 border-t border-zinc-200/50 dark:border-zinc-800/50 flex flex-wrap gap-2 text-xs font-mono">
                {#each Object.entries(currentParams) as [k, val] (k)}
                  {@const cleanK = k.replace(/^\./, '')}
                  {@const fieldName = fieldMap[cleanK] || fieldMap[k] || k}
                  <span class="rounded-lg bg-white dark:bg-zinc-950 px-3 py-1 border border-zinc-200 dark:border-zinc-800 text-zinc-600 dark:text-zinc-400 flex items-center gap-1.5 shadow-2xs">
                    <span class="text-blue-600 dark:text-blue-400 font-bold">{fieldName}:</span>
                    <strong class="text-zinc-900 dark:text-zinc-100">{String(val)}</strong>
                  </span>
                {:else}
                  <span class="text-xs text-zinc-400 italic">该成品使用纯静态或环境默认值。</span>
                {/each}
              </div>
            {/if}
          </div>
        {/each}
      {/if}
    </div>

    <!-- 底部状态条 -->
    <div class="h-8 border-t border-zinc-200 bg-zinc-50 px-5 flex items-center justify-between text-xs font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
      <div class="flex items-center gap-4">
        <span class="flex items-center gap-2">
          <span class="h-2 w-2 rounded-full {ws.compileError ? 'bg-rose-500' : 'bg-emerald-500'}"></span>
          <span class="font-bold">{ws.compileError ? '装配有误' : '流水线就绪'}</span>
        </span>
        {#if ws.activeCommitHash}
          <span>git:{ws.activeCommitHash.slice(0, 7)}</span>
        {/if}
      </div>
      <div class="flex items-center gap-4">
        <span>挂载积木: {ws.resolvedModuleBindings.length} 项</span>
        <span>涉及节点: {referencedNodes.length} 台</span>
      </div>
    </div>
  </main>

  <!-- 右侧编译镜像 -->
  <section class="flex w-[42%] min-w-95 max-w-155 shrink-0 flex-col bg-zinc-50 dark:bg-zinc-950/90 relative">
    <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
      <div class="flex items-center gap-2">
        <span class="font-mono text-sm font-bold text-zinc-500 dark:text-zinc-400">CONFIG MIRROR</span>
        {#if ws.activeCommitHash}
          <button onclick={() => ws.openTimeline()} type="button" class="flex items-center gap-1.5 rounded bg-zinc-200/80 px-2.5 py-0.5 font-mono text-xs font-semibold text-zinc-700 hover:bg-zinc-300 dark:bg-zinc-800 dark:text-zinc-300 cursor-pointer shadow-2xs">
            <span>git: {ws.activeCommitHash.slice(0, 7)}</span>
          </button>
        {/if}
      </div>

      <div class="flex items-center gap-2">
        <button onclick={copyCompiledJSON} type="button" class="rounded border border-zinc-300 bg-white px-3 py-1 text-xs font-mono font-bold text-zinc-700 shadow-2xs hover:border-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1.5 transition">
          <Icon name="copy" size={13} />
          <span>{copiedJSON ? '已复制' : '复制 JSON'}</span>
        </button>
        {#if ws.activeProfile?.capabilities.enable_subscribe}
          <button onclick={() => copySubURL(ws.activeProfile?.token ?? '')} type="button" class="rounded border border-zinc-300 bg-white px-3 py-1 text-xs font-mono font-bold text-zinc-700 shadow-2xs hover:border-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1.5 transition">
            <Icon name="link" size={13} />
            <span>{copiedSub ? '已复制' : '复制订阅'}</span>
          </button>
        {/if}
      </div>
    </div>

    <!-- 内部组装崩溃报错 (Fatal) -->
    {#if ws.compileError}
      <div class="m-3 p-4 rounded-xl bg-rose-50/80 text-rose-800 border border-rose-200/80 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-900/60 text-xs font-mono whitespace-pre-wrap overflow-y-auto max-h-48 shadow-inner">
        <div class="flex items-center gap-1.5 mb-2 font-bold">
          <Icon name="close" size={14} />
          <span>内部装配崩溃:</span>
        </div>
        {ws.compileError}
      </div>
    {/if}

    <!-- 双轨官方校验结果诊断条 (分页切换) -->
    {#if ws.validationReport?.enabled}
      {@const r = ws.validationReport}
      <div class="mx-3 mt-3 rounded-xl border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900 text-xs space-y-2.5 shadow-sm">
        <div class="flex items-center justify-between border-b border-zinc-100 pb-2.5 dark:border-zinc-800">
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold uppercase text-zinc-400">官方内核诊断:</span>
            
            {#if isDualTrack}
              <div class="flex items-center gap-1 bg-zinc-100 dark:bg-zinc-800 p-0.5 rounded-lg border border-zinc-200 dark:border-zinc-700">
                <button
                  onclick={() => activeLinterTab = 'stable'}
                  type="button"
                  class="px-3 py-1 rounded-md text-xs font-mono flex items-center gap-1.5 transition cursor-pointer {activeLinterTab === 'stable' ? 'bg-white shadow-2xs font-bold text-zinc-900 dark:bg-zinc-900 dark:text-white' : 'text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-300'}"
                >
                  <span class="h-2 w-2 rounded-full {r.stable?.valid ? 'bg-emerald-500' : 'bg-amber-500'}"></span>
                  <span>Stable ({r.stable?.version})</span>
                </button>

                <button
                  onclick={() => activeLinterTab = 'alpha'}
                  type="button"
                  class="px-3 py-1 rounded-md text-xs font-mono flex items-center gap-1.5 transition cursor-pointer {activeLinterTab === 'alpha' ? 'bg-white shadow-2xs font-bold text-zinc-900 dark:bg-zinc-900 dark:text-white' : 'text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-300'}"
                >
                  <span class="h-2 w-2 rounded-full {r.alpha?.valid ? 'bg-emerald-500' : 'bg-amber-500'}"></span>
                  <span>Alpha ({r.alpha?.version})</span>
                </button>
              </div>
            {:else if r.stable}
              <span class="px-3 py-1 rounded-lg text-xs font-mono flex items-center gap-1.5 bg-zinc-100 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 border border-zinc-200 dark:border-zinc-700">
                <span class="h-2 w-2 rounded-full {r.stable.valid ? 'bg-emerald-500' : 'bg-amber-500'}"></span>
                <span>Stable ({r.stable.version})</span>
              </span>
            {:else if r.alpha}
              <span class="px-3 py-1 rounded-lg text-xs font-mono flex items-center gap-1.5 bg-zinc-100 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300 border border-zinc-200 dark:border-zinc-700">
                <span class="h-2 w-2 rounded-full {r.alpha.valid ? 'bg-emerald-500' : 'bg-amber-500'}"></span>
                <span>Alpha ({r.alpha.version})</span>
              </span>
            {/if}
          </div>

          <button
            onclick={() => showDetailModal = true}
            type="button"
            class="text-xs font-mono text-blue-600 dark:text-blue-400 font-bold hover:underline cursor-pointer"
          >
            查看详情 ↗
          </button>
        </div>

        {#if currentTabResult}
          <div class="text-xs font-mono truncate py-0.5 {currentTabResult.valid ? 'text-emerald-600 dark:text-emerald-400 font-bold' : 'text-amber-600 dark:text-amber-400'}">
            {#if currentTabResult.valid}
              ✓ 语法检测完美通过
            {:else}
              ⚠️ 存在运行隐患: {currentTabResult.output.split('\n')[0]}
            {/if}
          </div>
        {/if}
      </div>
    {/if}

    <div class="flex-1 overflow-hidden p-3 pt-2">
      <div class="h-full rounded-2xl border border-zinc-200 bg-white p-1.5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950">
        <CodeEditor value={ws.compiledJSON || '// 暂无编译内容'} readonly={true} dark={ws.isDark} />
      </div>
    </div>

    <!-- 底部状态条 (高度增至 h-8) -->
    <div class="h-8 border-t border-zinc-200 bg-zinc-50 px-4 flex items-center justify-between text-xs font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
      <span>
        {#if !ws.validationReport?.enabled}
          <span class="text-zinc-600 dark:text-zinc-300 font-bold">JSON 语法 100% 合法 (内核检测已关闭)</span>
        {:else if ws.validationReport.stable?.valid && ws.validationReport.alpha?.valid}
          <span class="text-emerald-600 dark:text-emerald-400 font-bold">双轨内核 100% 校验通过</span>
        {:else if isDualTrack}
          <span class="text-amber-600 dark:text-amber-400 font-bold">版本存在语法差异</span>
        {:else if currentTabResult?.valid}
          <span class="text-emerald-600 dark:text-emerald-400 font-bold">内核语法检测通过</span>
        {:else}
          <span class="text-amber-600 dark:text-amber-400 font-bold">内核检测到隐患</span>
        {/if}
      </span>
      <span>大小: {(new Blob([ws.compiledJSON]).size / 1024).toFixed(1)} KB</span>
    </div>

    <!-- 详细报错浮动弹窗 -->
    {#if showDetailModal && ws.validationReport}
      {@const r = ws.validationReport}
      <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
        <div class="w-full max-w-3xl rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-5 max-h-[85vh] flex flex-col">
          <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800 shrink-0">
            <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
              <Icon name="terminal" size={17} color="blue" />
              <span>Sing-box 官方内核完整校验报告</span>
            </h3>
            <button onclick={() => showDetailModal = false} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
              <Icon name="close" size={15} />
            </button>
          </div>

          <div class="flex-1 overflow-y-auto space-y-5 text-sm font-mono pr-1">
            {#if r.stable}
              <div class="space-y-2">
                <span class="font-bold text-zinc-800 dark:text-zinc-200 block border-l-4 {r.stable.valid ? 'border-emerald-500' : 'border-amber-500'} pl-2">
                  稳定版检测产物 (Version: {r.stable.version})
                </span>
                <pre class="p-4 rounded-xl bg-zinc-50 dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-800 whitespace-pre-wrap {r.stable.valid ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'} shadow-inner">{r.stable.output}</pre>
              </div>
            {/if}

            {#if r.alpha}
              <div class="space-y-2">
                <span class="font-bold text-zinc-800 dark:text-zinc-200 block border-l-4 {r.alpha.valid ? 'border-emerald-500' : 'border-amber-500'} pl-2">
                  尝鲜版检测产物 (Version: {r.alpha.version})
                </span>
                <pre class="p-4 rounded-xl bg-zinc-50 dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-800 whitespace-pre-wrap {r.alpha.valid ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'} shadow-inner">{r.alpha.output}</pre>
              </div>
            {/if}
          </div>

          <div class="flex justify-end border-t border-zinc-100 pt-3 dark:border-zinc-800 shrink-0">
            <button onclick={() => showDetailModal = false} type="button" class="rounded-lg bg-blue-600 px-5 py-2 text-xs font-bold text-white hover:bg-blue-500 cursor-pointer shadow-sm">
              关闭详情
            </button>
          </div>
        </div>
      </div>
    {/if}
  </section>
</div>