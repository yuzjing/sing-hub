<!-- frontend/src/lib/components/workspaces/ModulesWorkspace.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Module, Component, Device } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ParamBinder from '$lib/components/ParamBinder.svelte';
  import TagFilter from '$lib/components/TagFilter.svelte';

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

  // 补齐积木标签数量统计
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
    const nodeIds = new Set<string>();
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
        {@const catModules = ws.modules.filter((m: Module) => m.category === cat.key && (selectedFilterTags.length === 0 || selectedFilterTags.some((t: string) => (m.tags || []).includes(t))))}
        {@const isCollapsed = collapsedCategories[cat.key] === true}
        
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
            <span class="text-[10px] font-mono px-2 py-0.5 rounded-full font-bold {catModules.length > 0 ? 'bg-zinc-200 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300' : 'bg-transparent text-zinc-400'}">
              {catModules.length}
            </span>
          </button>

          {#if !isCollapsed}
            <div class="mt-1 flex flex-col gap-1.5 px-1.5 pb-1.5 animate-in fade-in duration-150">
              {#each catModules as mod (mod.id)}
                {@const isSelected = selectedModule?.id === mod.id}
                <div class="group relative flex items-center justify-between rounded-xl px-3 py-2.5 text-xs transition border shadow-2xs {isSelected && !isModuleManageMode ? 'border-blue-500/50 bg-white text-zinc-900 font-extrabold dark:border-blue-500/40 dark:bg-zinc-900 dark:text-white' : 'border-zinc-200 bg-white/50 text-zinc-600 hover:bg-white dark:border-zinc-800 dark:bg-zinc-950/50 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
                  {#if isSelected && !isModuleManageMode}
                    <span class="absolute left-0 top-2 bottom-2 w-1.5 rounded-r-full bg-blue-500"></span>
                  {/if}

                  <div class="flex items-center gap-2.5 truncate flex-1 pl-1.5">
                    {#if isModuleManageMode}
                      <input
                        type="checkbox"
                        checked={selectedModuleIds.includes(mod.id)}
                        onchange={(e) => {
                          if ((e.target as HTMLInputElement).checked) selectedModuleIds = [...selectedModuleIds, mod.id];
                          else selectedModuleIds = selectedModuleIds.filter((id: string) => id !== mod.id);
                        }}
                        class="rounded h-3.5 w-3.5 cursor-pointer"
                      />
                    {/if}
                    <button onclick={() => { activeModuleId = mod.id; }} type="button" class="truncate text-left flex-1 cursor-pointer font-medium">
                      {mod.name}
                    </button>
                  </div>

                  <div class="flex items-center gap-1.5 shrink-0">
                    {#if !isModuleManageMode}
                      <span class="font-mono text-[10px] text-zinc-400 font-bold px-1.5 py-0.5 rounded-md border border-zinc-200 dark:border-zinc-700 bg-zinc-100 dark:bg-zinc-800">
                        {mod.template?.mount_path || 'path'}
                      </span>
                    {:else}
                      <button title="克隆积木" onclick={() => onCloneModule(mod)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                        <Icon name="copy" size={14} />
                      </button>
                      <button title="编辑元属性 (ID/名称/分类)" onclick={() => onEditModuleMeta(mod)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                        <Icon name="pencil" size={14} />
                      </button>
                      <button title="删除积木" onclick={() => triggerConfirm('删除成品积木？', '将永久删除 [' + mod.name + ']。引用此积木的方案将自动解绑。', () => ws.batchDeleteModules([mod.id]))} type="button" class="text-zinc-400 hover:text-rose-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800">
                        <Icon name="trash" size={14} />
                      </button>
                    {/if}
                  </div>
                </div>
              {:else}
                <span class="text-xs text-zinc-400 italic px-2 py-2 block text-center">暂无积木</span>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </aside>

  <!-- 右侧主从工作台 -->
  {#if selectedModule}
    <main class="flex flex-1 min-w-0 overflow-hidden bg-white dark:bg-zinc-950">
      <div class="flex flex-1 flex-col border-r border-zinc-200 dark:border-zinc-800">
        <!-- 头部导航 -->
        <div class="flex h-14 shrink-0 items-center justify-between border-b border-zinc-200 px-6 dark:border-zinc-800">
          <div class="flex items-center gap-3">
            <span class="text-sm font-extrabold text-zinc-900 dark:text-zinc-100">{selectedModule.name}</span>
            <span class="rounded-md border border-zinc-200 bg-zinc-100 px-2 py-0.5 font-mono text-xs font-bold text-zinc-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-400 shadow-2xs">
              ID: {selectedModule.id}
            </span>
            <span class="text-zinc-300 dark:text-zinc-700 text-lg mx-1">|</span>
            <span class="text-xs text-zinc-500 font-medium">母版:</span>
            <button
              onclick={() => {
                ws.switchWorkspace('foundry');
              }}
              type="button"
              class="text-xs text-blue-600 dark:text-blue-400 hover:underline font-bold cursor-pointer transition"
            >
              {targetTmpl?.name || selectedModule.template_id} ↗
            </button>
          </div>

          <div class="flex items-center gap-2.5">
            {#if isDirty}
              <button
                onclick={handleCancelChanges}
                type="button"
                class="rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-800 px-3.5 py-1.5 text-xs font-semibold text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-zinc-700 cursor-pointer transition shadow-2xs"
              >
                放弃修改
              </button>
            {/if}

            <button
              onclick={handleSave}
              type="button"
              class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-bold text-white shadow-sm hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5"
            >
              <Icon name="check" size={14} />
              <span>{isDirty ? '保存实参 *' : '保存积木'}</span>
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto p-6 space-y-6">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="rounded-2xl border border-zinc-200 bg-zinc-50/70 p-4 dark:border-zinc-800 dark:bg-zinc-900/60 space-y-2">
              <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300 flex items-center gap-1.5">
                <Icon name="layers" color="amber" size={14} />
                <span>装配此积木的方案 ({impactedProfiles.length})</span>
              </span>
              {#if impactedProfiles.length === 0}
                <span class="text-xs text-zinc-400 italic block py-1">暂无方案装配此积木</span>
              {:else}
                <div class="flex flex-wrap gap-2 pt-1">
                  {#each impactedProfiles as prof (prof.id)}
                    <button
                      onclick={() => {
                        ws.switchWorkspace('studio');
                        ws.selectProfile(prof.id);
                      }}
                      type="button"
                      class="rounded-lg border border-amber-200/80 bg-amber-50/60 px-3 py-1 text-[11px] font-bold text-amber-700 hover:bg-amber-100 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-300 transition cursor-pointer shadow-2xs"
                    >
                      {prof.name} ↗
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="rounded-2xl border border-zinc-200 bg-zinc-50/70 p-4 dark:border-zinc-800 dark:bg-zinc-900/60 space-y-2">
              <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300 flex items-center gap-1.5">
                <Icon name="server" color="blue" size={14} />
                <span>此积木绑定的目标节点 ({usedNodes.length})</span>
              </span>
              {#if usedNodes.length === 0}
                <span class="text-xs text-zinc-400 italic block py-1">使用静态值，未绑定任何节点</span>
              {:else}
                <div class="flex flex-wrap gap-2 pt-1">
                  {#each usedNodes as dev (dev.id)}
                    <button
                      onclick={() => {
                        ws.switchWorkspace('fleet');
                      }}
                      type="button"
                      class="rounded-lg border border-blue-200/80 bg-blue-50/60 px-3 py-1 text-[11px] font-bold text-blue-700 hover:bg-blue-100 dark:border-blue-900/60 dark:bg-blue-950/40 dark:text-blue-300 transition cursor-pointer shadow-2xs"
                    >
                      {dev.name} ↗
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </div>

          <div class="space-y-4">
            <div class="flex items-center justify-between">
              <span class="text-sm font-bold text-zinc-800 dark:text-zinc-200">
                固化实参配置 (母版插孔定义):
              </span>
            </div>

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

        <div class="h-8 border-t border-zinc-200 bg-zinc-50 px-4 flex items-center justify-between text-xs font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
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
      <div class="flex w-[45%] flex-col bg-zinc-50/50 dark:bg-zinc-950/80">
        <div class="flex h-14 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
          <span class="font-mono text-xs font-bold text-zinc-500 dark:text-zinc-400">MODULE PREVIEW (JSON CHUNK)</span>
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
        
        <!-- 错误面板呈现 -->
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
        <div class="h-8 border-t border-zinc-200 bg-zinc-50 px-4 flex items-center justify-between text-xs font-mono text-zinc-400 dark:border-zinc-800 dark:bg-zinc-900/80">
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