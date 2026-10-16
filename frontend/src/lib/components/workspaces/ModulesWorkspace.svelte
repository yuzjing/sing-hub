<!-- frontend/src/lib/components/workspaces/ModulesWorkspace.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Module, Component, Device } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import ParamBinder from '$lib/components/ParamBinder.svelte';

  let {
    activeModuleId = $bindable(''),
    isModuleManageMode = $bindable(false),
    selectedModuleIds = $bindable<string[]>([]),
    onEditModuleMeta = () => {},
    onCloneModule = () => {},
    onNewModule = () => {},
    triggerConfirm = () => {}
  } = $props<{
    activeModuleId?: string;
    isModuleManageMode?: boolean;
    selectedModuleIds?: string[];
    onEditModuleMeta?: (mod: Module) => void;
    onCloneModule?: (mod: Module) => void;
    onNewModule?: () => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }>();

  const CATEGORIES = [
    { key: 'io', title: '流量出入 (IO)' },
    { key: 'routing', title: '路由决策 (Routing)' },
    { key: 'dns', title: '域名解析 (DNS)' },
    { key: 'system', title: '系统基础设施 (System)' }
  ];

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
    return ws.profiles.filter(p => (p.modules || []).some(b => b.module_id === selectedModule.id));
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
    ws.showToast(`积木 [${selectedModule.name}] 已保存 ✓`, 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }

  function applyNodeRef(paramKey: string, devId: string) {
    draftParams[paramKey] = `@node:${devId}.endpoint`;
    isDirty = true;
    triggerPreviewDebounced();
  }
</script>

<div class="flex flex-1 min-h-0 overflow-hidden relative">
  <!-- 左侧：积木分类列表 -->
  <aside class="flex w-64 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50/70 dark:border-zinc-800 dark:bg-zinc-950/40">
    <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-3.5 dark:border-zinc-800">
      <span class="text-[11px] font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500">成品积木库 (BLOCKS)</span>
      <div class="flex items-center gap-1">
        <button
          title={isModuleManageMode ? '退出管理' : '批量整理积木'}
          onclick={() => { isModuleManageMode = !isModuleManageMode; selectedModuleIds = []; }}
          class="flex h-6 w-6 items-center justify-center rounded text-xs transition cursor-pointer {isModuleManageMode ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300 font-bold' : 'text-zinc-400 hover:bg-zinc-200/50 hover:text-zinc-700 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
        >
          <Icon name="sliders" size={14} />
        </button>
        <button
          title="新建成品积木"
          onclick={onNewModule}
          class="flex h-6 w-6 items-center justify-center rounded text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/50 cursor-pointer"
        >
          <Icon name="plus" size={15} />
        </button>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto p-2 space-y-4">
      {#each CATEGORIES as cat (cat.key)}
        <div>
          <span class="text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider px-2 block mb-1">
            {cat.title}
          </span>
          <div class="flex flex-col gap-1">
            {#each ws.modules.filter((m: Module) => m.category === cat.key) as mod (mod.id)}
              <div class="group flex items-center justify-between rounded-lg px-2.5 py-2 text-xs transition {selectedModule?.id === mod.id && !isModuleManageMode ? 'bg-white shadow-xs ring-1 ring-zinc-300 text-zinc-900 font-semibold dark:bg-zinc-800 dark:text-white' : 'text-zinc-600 hover:bg-zinc-200/50 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
                <div class="flex items-center gap-2 truncate flex-1">
                  {#if isModuleManageMode}
                    <input
                      type="checkbox"
                      checked={selectedModuleIds.includes(mod.id)}
                      onchange={(e) => {
                        if ((e.target as HTMLInputElement).checked) selectedModuleIds = [...selectedModuleIds, mod.id];
                        else selectedModuleIds = selectedModuleIds.filter((id: string) => id !== mod.id);
                      }}
                      class="rounded cursor-pointer"
                    />
                  {/if}
                  <button onclick={() => { activeModuleId = mod.id; }} class="truncate text-left flex-1 cursor-pointer">
                    {mod.name}
                  </button>
                </div>

                <div class="flex items-center gap-1">
                  {#if !isModuleManageMode}
                    <span class="font-mono text-[9px] text-zinc-400 truncate max-w-16">[{mod.template?.mount_path || 'path'}]</span>
                  {:else}
                    <button title="克隆积木" onclick={() => onCloneModule(mod)} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                      <Icon name="copy" size={13} />
                    </button>
                    <button title="编辑元属性 (ID/名称/分类)" onclick={() => onEditModuleMeta(mod)} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                      <Icon name="pencil" size={13} />
                    </button>
                    <button title="删除积木" onclick={() => triggerConfirm('删除成品积木？', `将永久删除 [${mod.name}]。所有引用该积木的方案将自动解绑。`, () => ws.batchDeleteModules([mod.id]))} class="text-zinc-400 hover:text-rose-500 cursor-pointer p-0.5">
                      <Icon name="trash" size={13} />
                    </button>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  </aside>

  <!-- 右侧主从工作台 -->
  {#if selectedModule}
    <main class="flex flex-1 min-w-0 overflow-hidden bg-white dark:bg-zinc-950">
      <div class="flex flex-1 flex-col border-r border-zinc-200 dark:border-zinc-800">
        <!-- 头部导航 -->
        <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
          <div class="flex items-center gap-2">
            <span class="text-xs font-bold text-zinc-900 dark:text-zinc-100">{selectedModule.name}</span>
            <span class="rounded bg-zinc-100 px-1.5 py-0.2 font-mono text-[10px] text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
              ID: {selectedModule.id}
            </span>
            <span class="text-zinc-300 dark:text-zinc-700">|</span>
            <span class="text-xs text-zinc-400">母版:</span>
            <button
              onclick={() => {
                ws.switchWorkspace('foundry');
              }}
              class="text-xs text-blue-600 hover:underline font-medium cursor-pointer"
            >
              {targetTmpl?.name || selectedModule.template_id} ↗
            </button>
          </div>

          <div class="flex items-center gap-2">
            {#if isDirty}
              <button
                onclick={handleCancelChanges}
                class="rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-800 px-2.5 py-1 text-xs text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-zinc-700 cursor-pointer transition"
              >
                放弃修改
              </button>
            {/if}

            <button
              onclick={handleSave}
              class="rounded-md bg-blue-600 px-3.5 py-1 text-xs font-semibold text-white shadow-2xs hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5"
            >
              <Icon name="check" size={13} />
              <span>{isDirty ? '保存实参 *' : '保存积木'}</span>
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto p-6 space-y-6">
          <!-- 受影响方案透视 -->
          <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-4 dark:border-zinc-800 dark:bg-zinc-900/60 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300 flex items-center gap-1.5">
                <Icon name="layers" color="amber" size={14} />
                <span>正在引用此积木的方案 ({impactedProfiles.length})</span>
              </span>
              <span class="text-[10px] text-zinc-400">实参变动将直接影响上述方案的编译产物</span>
            </div>

            {#if impactedProfiles.length === 0}
              <div class="text-[11px] text-zinc-400 italic py-1">当前暂无方案装配此积木。</div>
            {:else}
              <div class="flex flex-wrap gap-2 pt-1">
                {#each impactedProfiles as prof (prof.id)}
                  <button
                    onclick={() => {
                      ws.switchWorkspace('studio');
                      ws.selectProfile(prof.id);
                    }}
                    class="rounded-lg border border-amber-200/80 bg-amber-50/60 px-2.5 py-1 text-xs font-medium text-amber-700 hover:bg-amber-100 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-300 transition cursor-pointer flex items-center gap-1.5"
                    title="点击前往方案装配大盘"
                  >
                    <span>{prof.name}</span>
                    <span class="text-[10px] opacity-60">↗</span>
                  </button>
                {/each}
              </div>
            {/if}
          </div>

          <!-- 实参配置：使用通用的 ParamBinder 组件，拥有强大的层级树节点端点引用能力 -->
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300">
                固化实参配置 (母版插孔定义):
              </span>
            </div>

            <ParamBinder
              {templateVars}
              bind:params={draftParams}
              onChange={() => {
                isDirty = true;
                triggerPreviewDebounced();
              }}
            />
          </div>
        </div>

        <!-- 底部状态条 (Status Bar) -->
        <div class="h-7 border-t border-zinc-200 bg-zinc-50 px-3 flex items-center justify-between text-[11px] font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
          <div class="flex items-center gap-3">
            <span class="flex items-center gap-1.5">
              <span class="h-1.5 w-1.5 rounded-full {isDirty ? 'bg-amber-400' : 'bg-emerald-500'}"></span>
              <span>{isDirty ? '未保存修改' : '已保存同步'}</span>
            </span>
            <span>保存于: {lastSavedAt}</span>
          </div>
          <div class="flex items-center gap-3">
            <span>实参数量: {templateVars.length} 项</span>
            <span>受影响方案: {impactedProfiles.length} 个</span>
          </div>
        </div>
      </div>

      <!-- 右侧：真实单体 JSON 沙箱 -->
      <div class="flex w-[45%] flex-col bg-zinc-50/50 dark:bg-zinc-950/80">
        <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
          <span class="font-mono text-xs font-semibold text-zinc-500">MODULE PREVIEW (JSON CHUNK)</span>
          {#if previewError}
            <span class="text-[11px] text-rose-500 font-mono font-bold flex items-center gap-1" title={previewError}>
              <Icon name="close" size={12} />
              <span>语法错误</span>
            </span>
          {:else}
            <span class="text-[10px] text-emerald-500 font-semibold flex items-center gap-1">
              <Icon name="check" size={12} />
              <span>单体合法 JSON</span>
            </span>
          {/if}
        </div>
        <div class="flex-1 overflow-hidden p-3">
          <div class="h-full rounded-xl border border-zinc-200 bg-white p-1 shadow-2xs dark:border-zinc-800/80 dark:bg-zinc-950">
            <CodeEditor
              value={renderedChunkJSON}
              readonly={true}
              dark={ws.isDark}
            />
          </div>
        </div>
        <div class="h-7 border-t border-zinc-200 bg-zinc-50 px-3 flex items-center justify-between text-[11px] font-mono text-zinc-400 dark:border-zinc-800 dark:bg-zinc-900/80">
          <span>挂载目标: {targetTmpl?.mount_path || '未指定'}</span>
          <span>策略: {targetTmpl?.strategy || 'append'}</span>
        </div>
      </div>
    </main>
  {:else}
    <div class="flex-1 flex items-center justify-center text-zinc-400 text-xs">
      暂无成品积木，请点击左上角「+」新建积木
    </div>
  {/if}
</div>