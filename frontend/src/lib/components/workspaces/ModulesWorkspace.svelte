<!-- frontend/src/lib/components/workspaces/ModulesWorkspace.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Module, Component, Device } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ParamBinder from '$lib/components/ParamBinder.svelte';
  import TagFilter from '$lib/components/TagFilter.svelte';
  import { SvelteSet } from 'svelte/reactivity';

  interface Props {
    activeModuleId?: string;
    isModuleManageMode?: boolean;
    selectedModuleIds?: string[];
    onEditModuleMeta?: (mod: Module) => void;
    onCloneModule?: (mod: Module) => void;
    onNewModule?: (e?: MouseEvent) => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }

  let {
    activeModuleId = $bindable(''),
    isModuleManageMode = $bindable(false),
    selectedModuleIds = $bindable<string[]>([]),
    onEditModuleMeta = () => {},
    onCloneModule = () => {},
    onNewModule = () => {},
    triggerConfirm = () => {}
  }: Props = $props();

  interface CategoryDef {
    key: string;
    title: string;
    color: 'blue' | 'amber' | 'emerald' | 'purple';
    icon: 'layers' | 'sliders' | 'boxes' | 'server';
  }

  const CATEGORIES: CategoryDef[] = [
    { key: 'io', title: '流量出入 (IO)', color: 'blue', icon: 'layers' },
    { key: 'routing', title: '路由决策 (ROUTING)', color: 'amber', icon: 'sliders' },
    { key: 'dns', title: '域名解析 (DNS)', color: 'emerald', icon: 'boxes' },
    { key: 'system', title: '基础设施 (SYSTEM)', color: 'purple', icon: 'server' }
  ];

  let collapsedCategories = $state<Record<string, boolean>>({});
  let selectedFilterTags = $state<string[]>([]);

  function toggleCategory(catKey: string) {
    collapsedCategories[catKey] = !collapsedCategories[catKey];
  }

  let tagCounts = $derived.by(() => {
    const counts: Record<string, number> = {};
    ws.modules.forEach((m: Module) => {
      (m.tags || []).forEach((t: string) => {
        counts[t] = (counts[t] || 0) + 1;
      });
    });
    return counts;
  });
  let availableTags = $derived.by(() => Object.keys(tagCounts));

  let selectedModule = $derived.by(() => {
    return ws.modules.find((m: Module) => m.id === activeModuleId) || ws.modules[0] || null;
  });

  let targetTmpl = $derived.by(() => {
    if (!selectedModule) return null;
    return ws.components.find((c: Component) => c.id === selectedModule.template_id) || null;
  });

  let templateVars = $derived.by(() => {
    if (!targetTmpl || !targetTmpl.payload_tmpl) return [];
    const matches = targetTmpl.payload_tmpl.matchAll(/\{\{\s*\.([a-zA-Z0-9_.]+)(?:\s*\|[^}]+)?\s*\}\}/g);
    const unique: string[] = [];
    for (const m of matches) {
      if (!unique.includes(m[1])) unique.push(m[1]);
    }
    return unique;
  });

  let impactedProfiles = $derived.by(() => {
    if (!selectedModule) return [];
    return ws.profiles.filter((p: { modules?: Array<{ module_id: string }> }) => 
      (p.modules || []).some(b => b.module_id === selectedModule.id)
    );
  });

  let usedNodes = $derived.by(() => {
    if (!draftParams) return [];
    const nodeIds = new SvelteSet<string>();
    for (const val of Object.values(draftParams)) {
      const s = String(val);
      if (s.startsWith('@node:')) {
        const parts = s.replace('@node:', '').split('.');
        if (parts.length > 0) nodeIds.add(parts[0]);
      }
    }
    return ws.devices.filter((d: Device) => nodeIds.has(d.id));
  });

  let draftParams = $state<Record<string, string | number | boolean>>({});
  let isDirty = $state(false);
  let lastSavedAt = $state('已同步');
  let renderedChunkJSON = $state('// 实时渲染中...');
  let previewError = $state('');
  let previewTimer: ReturnType<typeof setTimeout> | null = null;
  let lastLoadedModId = '';

  function safeParse(raw: string | undefined): Record<string, string | number | boolean> {
    if (!raw) return {};
    try {
      return JSON.parse(raw);
    } catch {
      return {};
    }
  }

  const HASH_PALETTES = [
    { bg: 'bg-emerald-500/10 text-emerald-700 border-emerald-500/25 dark:text-emerald-300 dark:border-emerald-500/30', dot: 'bg-emerald-500' },
    { bg: 'bg-blue-500/10 text-blue-700 border-blue-500/25 dark:text-blue-300 dark:border-blue-500/30', dot: 'bg-blue-500' },
    { bg: 'bg-purple-500/10 text-purple-700 border-purple-500/25 dark:text-purple-300 dark:border-purple-500/30', dot: 'bg-purple-500' },
    { bg: 'bg-amber-500/10 text-amber-700 border-amber-500/25 dark:text-amber-300 dark:border-amber-500/30', dot: 'bg-amber-500' },
    { bg: 'bg-cyan-500/10 text-cyan-700 border-cyan-500/25 dark:text-cyan-300 dark:border-cyan-500/30', dot: 'bg-cyan-500' },
    { bg: 'bg-rose-500/10 text-rose-700 border-rose-500/25 dark:text-rose-300 dark:border-rose-500/30', dot: 'bg-rose-500' },
    { bg: 'bg-indigo-500/10 text-indigo-700 border-indigo-500/25 dark:text-indigo-300 dark:border-indigo-500/30', dot: 'bg-indigo-500' }
  ];

  function getMountMeta(path: string | undefined): { label: string; pillClass: string; dotClass: string } {
    if (!path) return { label: 'PATH', pillClass: 'bg-zinc-100 text-zinc-600 border-zinc-200 dark:bg-zinc-800 dark:text-zinc-400 dark:border-zinc-700', dotClass: 'bg-zinc-400' };

    const lower = path.toLowerCase();
    if (lower.startsWith('inbound')) return { label: path.toUpperCase(), pillClass: HASH_PALETTES[0].bg, dotClass: HASH_PALETTES[0].dot };
    if (lower.startsWith('outbound')) return { label: path.toUpperCase(), pillClass: HASH_PALETTES[1].bg, dotClass: HASH_PALETTES[1].dot };
    if (lower.startsWith('endpoint')) return { label: path.toUpperCase(), pillClass: HASH_PALETTES[2].bg, dotClass: HASH_PALETTES[2].dot };
    if (lower.startsWith('route')) return { label: path.toUpperCase(), pillClass: HASH_PALETTES[3].bg, dotClass: HASH_PALETTES[3].dot };
    if (lower.startsWith('dns')) return { label: path.toUpperCase(), pillClass: HASH_PALETTES[4].bg, dotClass: HASH_PALETTES[4].dot };

    let hash = 0;
    for (let i = 0; i < path.length; i++) {
      hash = (hash << 5) - hash + path.charCodeAt(i);
      hash |= 0;
    }
    const idx = Math.abs(hash) % HASH_PALETTES.length;
    return {
      label: path.toUpperCase(),
      pillClass: HASH_PALETTES[idx].bg,
      dotClass: HASH_PALETTES[idx].dot
    };
  }

  function groupModulesByMount(modules: Module[]) {
    const record: Record<string, Module[]> = {};
    for (const m of modules) {
      const p = m.template?.mount_path || 'other';
      if (!record[p]) record[p] = [];
      record[p].push(m);
    }

    const sortOrder = ['inbounds', 'outbounds', 'endpoints', 'route.rules', 'dns.servers'];
    const keys = Object.keys(record).sort((a, b) => {
      const ia = sortOrder.indexOf(a);
      const ib = sortOrder.indexOf(b);
      if (ia !== -1 && ib !== -1) return ia - ib;
      if (ia !== -1) return -1;
      if (ib !== -1) return 1;
      return a.localeCompare(b);
    });

    return keys.map((k) => ({
      mount: k,
      meta: getMountMeta(k),
      items: record[k]
    }));
  }

  let selectedModuleMeta = $derived.by(() => {
    return getMountMeta(targetTmpl?.mount_path);
  });

  $effect(() => {
    const curMod = selectedModule;
    if (curMod && curMod.id !== lastLoadedModId) {
      lastLoadedModId = curMod.id;
      draftParams = safeParse(curMod.params_json);
      isDirty = false;
      lastSavedAt = curMod.updated_at ? new Date(curMod.updated_at).toLocaleTimeString() : '已同步';
      updatePreviewDirect();
    }
  });

  function triggerPreviewDebounced() {
    if (previewTimer) clearTimeout(previewTimer);
    previewTimer = setTimeout(() => {
      updatePreviewDirect();
    }, 250);
  }

  async function updatePreviewDirect() {
    if (!selectedModule || !targetTmpl) return;
    if (!targetTmpl.payload_tmpl.trim()) {
      renderedChunkJSON = '{\n}';
      previewError = '';
      return;
    }
    try {
      renderedChunkJSON = await api.previewComponent(targetTmpl.payload_tmpl, JSON.stringify(draftParams), '');
      previewError = '';
    } catch (e: unknown) {
      previewError = e instanceof Error ? e.message : '渲染错误';
    }
  }

  function handleCancelChanges() {
    if (!selectedModule) return;
    draftParams = safeParse(selectedModule.params_json);
    isDirty = false;
    updatePreviewDirect();
    ws.showToast('已放弃未保存的实参修改', 'info');
  }

  async function handleSave() {
    if (!selectedModule) return;
    selectedModule.params_json = JSON.stringify(draftParams);
    await api.saveModule(selectedModule);
    await ws.loadModules();
    await ws.loadProfiles();
    isDirty = false;
    lastSavedAt = new Date().toLocaleTimeString();
    ws.showToast('积木 [' + selectedModule.name + '] 已保存 ✓', 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }
</script>

<div class="flex flex-1 min-h-0 overflow-hidden relative">
  <!-- 左栏：手风琴分类分组面板 -->
  <aside class="flex w-72 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50/70 dark:border-zinc-800 dark:bg-zinc-950/40">
    <div class="flex h-14 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
      <div class="flex items-center gap-2">
        <Icon name="boxes" size={16} color="blue" />
        <span class="text-sm font-bold uppercase tracking-wider text-zinc-600 dark:text-zinc-300">成品积木库 ({ws.modules.length})</span>
      </div>
      <div class="flex items-center gap-1.5">
        <button
          title={isModuleManageMode ? '退出管理' : '批量整理积木'}
          onclick={() => { isModuleManageMode = !isModuleManageMode; selectedModuleIds = []; }}
          type="button"
          class="flex h-7 w-7 items-center justify-center rounded-lg transition cursor-pointer shadow-2xs {isModuleManageMode ? 'bg-amber-100 text-amber-700 border border-amber-300 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-800' : 'bg-white border border-zinc-200 text-zinc-500 hover:bg-zinc-100 dark:bg-zinc-900 dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800'}"
        >
          <Icon name="sliders" size={14} />
        </button>
        <button
          title="新建成品积木"
          onclick={onNewModule}
          type="button"
          class="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-50 border border-blue-200 text-blue-600 hover:bg-blue-100 shadow-2xs dark:bg-blue-950/50 dark:border-blue-900/50 dark:text-blue-400 cursor-pointer transition"
        >
          <Icon name="plus" size={15} />
        </button>
      </div>
    </div>

    <!-- 标签多选筛选 -->
    <TagFilter {availableTags} {tagCounts} bind:selectedTags={selectedFilterTags} />

    <div class="flex-1 overflow-y-auto p-3 space-y-4">
      {#each CATEGORIES as cat (cat.key)}
        {@const filteredModules = ws.modules.filter((m: Module) => m.category === cat.key && (selectedFilterTags.length === 0 || selectedFilterTags.some((t: string) => (m.tags || []).includes(t))))}
        {@const isCollapsed = collapsedCategories[cat.key] === true}
        {@const subGroups = groupModulesByMount(filteredModules)}
        
        <div class="rounded-xl border border-zinc-200/70 bg-zinc-100/60 p-1.5 dark:border-zinc-800/80 dark:bg-zinc-900/50">
          <button
            onclick={() => toggleCategory(cat.key)}
            type="button"
            class="w-full flex items-center justify-between px-2.5 py-2 rounded-lg hover:bg-zinc-200/50 dark:hover:bg-zinc-800/60 transition cursor-pointer select-none text-left"
          >
            <div class="flex items-center gap-2">
              <span class="text-zinc-400 text-[10px] transform transition-transform duration-200 {isCollapsed ? '' : 'rotate-90'}">▶</span>
              <Icon name={cat.icon} size={14} color={cat.color} />
              <span class="text-xs font-bold text-zinc-800 dark:text-zinc-200">{cat.title}</span>
            </div>
            
            <!-- 一级总数微拟态徽标：>0 清晰显眼，=0 优雅淡化 -->
            <span class="inline-flex min-w-5 h-5 items-center justify-center text-[11px] font-mono font-bold px-1.5 rounded-full border shadow-2xs transition-all {filteredModules.length > 0 ? 'bg-zinc-200/90 border-zinc-300/80 text-zinc-800 dark:bg-zinc-800 dark:border-zinc-700 dark:text-zinc-200' : 'bg-transparent border-transparent text-zinc-400 opacity-40'}">
              {filteredModules.length}
            </span>
          </button>

          {#if !isCollapsed}
            <div class="mt-1 flex flex-col gap-2.5 px-1 pb-1 animate-in fade-in duration-150">
              {#if filteredModules.length === 0}
                <span class="text-xs text-zinc-400 italic px-2 py-2 block text-center">暂无积木</span>
              {:else}
                {#each subGroups as group (group.mount)}
                  <div class="space-y-1">
                    <!-- 二级标头：微拟态彩色胶囊 + 统一柔和计数微标 -->
                    <div class="flex items-center justify-between px-1.5 py-1">
                      <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border text-[10px] font-mono font-bold tracking-wide {group.meta.pillClass}">
                        <span class="w-1.5 h-1.5 rounded-full {group.meta.dotClass}"></span>
                        <span>{group.meta.label}</span>
                      </span>

                      <!-- 二级小分组数量微标：清晰可读，不再是隐形灰字 -->
                      <span class="inline-flex min-w-4.5 h-4.5 items-center justify-center text-[10px] font-mono font-bold px-1 rounded-md bg-zinc-200/70 border border-zinc-300/60 text-zinc-700 dark:bg-zinc-800 dark:border-zinc-700 dark:text-zinc-300 shadow-2xs">
                        {group.items.length}
                      </span>
                    </div>

                    {#each group.items as mod (mod.id)}
                      {@const isSelected = selectedModule?.id === mod.id}
                      {@const itemMeta = getMountMeta(mod.template?.mount_path)}
                      <div class="group relative flex items-center justify-between rounded-xl px-3 py-2 text-xs transition border shadow-2xs {isSelected && !isModuleManageMode ? 'border-blue-500/60 bg-white text-zinc-900 font-extrabold dark:border-blue-500/40 dark:bg-zinc-900 dark:text-white' : 'border-zinc-200 bg-white/60 text-zinc-600 hover:bg-white dark:border-zinc-800 dark:bg-zinc-950/50 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
                        {#if isSelected && !isModuleManageMode}
                          <span class="absolute left-0 top-2 bottom-2 w-1.5 rounded-r-full bg-blue-500"></span>
                        {/if}

                        <div class="flex items-center gap-2 truncate flex-1 pl-1">
                          {#if isModuleManageMode}
                            <input
                              type="checkbox"
                              checked={selectedModuleIds.includes(mod.id)}
                              onchange={(e) => {
                                if ((e.target as HTMLInputElement).checked) selectedModuleIds = [...selectedModuleIds, mod.id];
                                else selectedModuleIds = selectedModuleIds.filter((id: string) => id !== mod.id);
                              }}
                              class="rounded h-3.5 w-3.5 cursor-pointer shrink-0"
                            />
                          {/if}
                          <button onclick={() => { activeModuleId = mod.id; }} type="button" class="truncate text-left flex-1 cursor-pointer font-medium">
                            {mod.name}
                          </button>
                        </div>

                        <!-- 积木右侧直接继承母版的挂载点小胶囊 -->
                        <div class="flex items-center gap-1.5 shrink-0">
                          {#if !isModuleManageMode}
                            <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded border text-[9px] font-mono font-bold {itemMeta.pillClass}">
                              <span class="w-1 h-1 rounded-full {itemMeta.dotClass}"></span>
                              <span>{mod.template?.mount_path || 'path'}</span>
                            </span>
                            {#if mod.template?.sing_box_version && mod.template.sing_box_version !== 'universal'}
                              <span class="px-1.5 py-0.5 rounded bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400 font-mono text-[9px] font-bold">
                                v{mod.template.sing_box_version}
                              </span>
                            {/if}
                          {:else}
                            <button title="克隆积木" onclick={() => onCloneModule(mod)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                              <Icon name="copy" size={13} />
                            </button>
                            <button title="编辑元属性" onclick={() => onEditModuleMeta(mod)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                              <Icon name="pencil" size={13} />
                            </button>
                            <button title="删除积木" onclick={() => triggerConfirm('删除成品积木？', '将永久删除 [' + mod.name + ']。引用此积木的方案将自动解绑。', () => ws.batchDeleteModules([mod.id]))} type="button" class="text-zinc-400 hover:text-rose-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                              <Icon name="trash" size={13} />
                            </button>
                          {/if}
                        </div>
                      </div>
                    {/each}
                  </div>
                {/each}
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </aside>

  <!-- 右侧主工作台 -->
  {#if selectedModule}
    <main class="flex flex-1 min-w-0 overflow-hidden bg-white dark:bg-zinc-950">
      <div class="flex flex-1 min-w-0 flex-col border-r border-zinc-200 dark:border-zinc-800">
        <!-- 头部导航 -->
        <div class="flex h-14 shrink-0 items-center justify-between border-b border-zinc-200 px-6 dark:border-zinc-800 gap-4">
          <div class="flex items-center gap-3 min-w-0 flex-1">
            <span class="text-sm font-extrabold text-zinc-900 dark:text-zinc-100 truncate max-w-50" title={selectedModule.name}>
              {selectedModule.name}
            </span>
            <span class="rounded-md border border-zinc-200 bg-zinc-100 px-2 py-0.5 font-mono text-xs font-bold text-zinc-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-400 shadow-2xs shrink-0">
              ID: {selectedModule.id}
            </span>
            <span class="text-zinc-300 dark:text-zinc-700 text-lg mx-0.5 shrink-0">|</span>
            
            <div class="flex items-center gap-1.5 min-w-0 truncate">
              <span class="text-xs text-zinc-400 font-medium shrink-0">母版:</span>
              <button
                onclick={() => ws.switchWorkspace('foundry')}
                type="button"
                class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-md border text-xs font-mono font-bold tracking-wide transition cursor-pointer hover:opacity-85 shadow-2xs truncate {selectedModuleMeta.pillClass}"
                title={targetTmpl?.name || selectedModule.template_id}
              >
                <span class="w-1.5 h-1.5 rounded-full {selectedModuleMeta.dotClass} shrink-0"></span>
                <span class="truncate">{targetTmpl?.name || selectedModule.template_id}</span>
                <span class="shrink-0 text-[10px]">↗</span>
              </button>

              {#if targetTmpl?.sing_box_version && targetTmpl.sing_box_version !== 'universal'}
                <span class="px-1.5 py-0.5 rounded-md bg-zinc-100 text-zinc-600 border border-zinc-200 dark:bg-zinc-800 dark:text-zinc-400 dark:border-zinc-700 text-[9px] font-mono font-bold shrink-0">
                  v{targetTmpl.sing_box_version}
                </span>
              {/if}
            </div>
          </div>

          <div class="flex items-center gap-2 shrink-0">
            {#if isDirty}
              <button
                onclick={handleCancelChanges}
                type="button"
                class="rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-800 px-3 py-1.5 text-xs font-semibold text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-zinc-700 cursor-pointer transition shadow-2xs whitespace-nowrap"
              >
                放弃修改
              </button>
            {/if}

            <button
              onclick={handleSave}
              type="button"
              class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-bold text-white shadow-sm hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5 whitespace-nowrap"
            >
              <Icon name="check" size={14} />
              <span>{isDirty ? '保存实参 *' : '保存积木'}</span>
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto p-6 space-y-6">
          <!-- 拓扑引用卡片 -->
          <div class="grid grid-cols-1 xl:grid-cols-2 gap-4">
            <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-3.5 dark:border-zinc-800 dark:bg-zinc-900/60 space-y-2 min-w-0">
              <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300 flex items-center gap-1.5">
                <Icon name="layers" color="amber" size={14} />
                <span>装配此积木的方案 ({impactedProfiles.length})</span>
              </span>
              {#if impactedProfiles.length === 0}
                <span class="text-xs text-zinc-400 italic block py-0.5">暂无方案装配此积木</span>
              {:else}
                <div class="flex flex-wrap gap-1.5 pt-0.5">
                  {#each impactedProfiles as prof (prof.id)}
                    <button
                      onclick={() => {
                        ws.switchWorkspace('studio');
                        ws.selectProfile(prof.id);
                      }}
                      type="button"
                      class="rounded-lg border border-amber-200/80 bg-amber-50/60 px-2.5 py-0.5 text-[11px] font-bold text-amber-700 hover:bg-amber-100 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-300 transition cursor-pointer shadow-2xs truncate max-w-45"
                    >
                      {prof.name} ↗
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-3.5 dark:border-zinc-800 dark:bg-zinc-900/60 space-y-2 min-w-0">
              <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300 flex items-center gap-1.5">
                <Icon name="server" color="blue" size={14} />
                <span>此积木绑定的目标节点 ({usedNodes.length})</span>
              </span>
              {#if usedNodes.length === 0}
                <span class="text-xs text-zinc-400 italic block py-0.5">使用纯静态值，未绑定任何节点</span>
              {:else}
                <div class="flex flex-wrap gap-1.5 pt-0.5">
                  {#each usedNodes as dev (dev.id)}
                    <button
                      onclick={() => ws.switchWorkspace('fleet')}
                      type="button"
                      class="rounded-lg border border-blue-200/80 bg-blue-50/60 px-2.5 py-0.5 text-[11px] font-bold text-blue-700 hover:bg-blue-100 dark:border-blue-900/60 dark:bg-blue-950/40 dark:text-blue-300 transition cursor-pointer shadow-2xs truncate max-w-45"
                    >
                      {dev.name} ↗
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </div>

          <div class="space-y-4">
            <span class="text-sm font-bold text-zinc-800 dark:text-zinc-200 block">
              固化实参配置:
            </span>

            <ParamBinder
              {templateVars}
              templateCode={targetTmpl?.payload_tmpl || ''}
              bind:params={draftParams}
              onChange={() => {
                isDirty = true;
                triggerPreviewDebounced();
              }}
            />
          </div>
        </div>

        <div class="h-8 border-t border-zinc-200 bg-zinc-50 px-4 flex items-center justify-between text-xs font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80 shrink-0">
          <div class="flex items-center gap-3">
            <span class="flex items-center gap-1.5">
              <span class="h-2 w-2 rounded-full {isDirty ? 'bg-amber-400 shadow-[0_0_6px_#fbbf24]' : 'bg-emerald-500'}"></span>
              <span class="font-bold">{isDirty ? '未保存修改' : '已保存同步'}</span>
            </span>
            <span>保存于: {lastSavedAt}</span>
          </div>
          <div class="flex items-center gap-4">
            <span>实参数量: {templateVars.length} 项</span>
            <span>绑定节点: {usedNodes.length} 台</span>
          </div>
        </div>
      </div>

      <!-- 右侧：真实单体 JSON 沙箱 -->
      <div class="flex w-[42%] min-w-88 flex-col bg-zinc-50/50 dark:bg-zinc-950/80 shrink-0">
        <div class="flex h-14 shrink-0 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
          <span class="font-mono text-xs font-bold text-zinc-500 dark:text-zinc-400">MODULE PREVIEW</span>
          {#if previewError}
            <span class="text-[11px] text-rose-600 font-mono font-bold flex items-center gap-1.5 bg-rose-50 dark:bg-rose-950/40 px-2.5 py-1 rounded-lg border border-rose-200 dark:border-rose-900/50 shadow-2xs">
              <Icon name="close" size={13} />
              <span>语法错误</span>
            </span>
          {:else}
            <span class="text-[11px] text-emerald-600 font-bold flex items-center gap-1.5 bg-emerald-50 dark:bg-emerald-950/40 px-2.5 py-1 rounded-lg border border-emerald-200 dark:border-emerald-900/50 shadow-2xs">
              <Icon name="check" size={13} />
              <span>单体合法 JSON</span>
            </span>
          {/if}
        </div>
        
        {#if previewError}
          <div class="m-4 p-4 rounded-xl bg-rose-50/80 text-rose-800 border border-rose-200/80 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-900/60 text-xs font-mono whitespace-pre-wrap overflow-y-auto max-h-48 shadow-inner transition-all">
            <div class="flex items-center gap-1.5 mb-2 font-bold text-rose-700 dark:text-rose-400">
              <Icon name="close" size={14} />
              <span>JSON 模板渲染或校验失败:</span>
            </div>
            {previewError}
          </div>
        {/if}

        <div class="flex-1 overflow-hidden p-4 pt-1">
          <div class="h-full rounded-2xl border border-zinc-200 bg-white p-1.5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950">
            <CodeEditor
              value={renderedChunkJSON}
              readonly={true}
              dark={ws.isDark}
            />
          </div>
        </div>
        <div class="h-8 shrink-0 border-t border-zinc-200 bg-zinc-50 px-4 flex items-center justify-between text-xs font-mono text-zinc-400 dark:border-zinc-800 dark:bg-zinc-900/80">
          <span>挂载目标: {targetTmpl?.mount_path || '未指定'}</span>
          <span>策略: {targetTmpl?.strategy || 'append'}</span>
        </div>
      </div>
    </main>
  {:else}
    <div class="flex-1 flex items-center justify-center text-zinc-400 text-sm font-medium">
      暂无成品积木，请点击左上角「+」新建积木
    </div>
  {/if}
</div>