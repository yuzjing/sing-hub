<!-- frontend/src/lib/components/workspaces/FoundryWorkspace.svelte -->
<script lang="ts">
  import { untrack } from 'svelte';
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Component } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import TagFilter from '$lib/components/TagFilter.svelte';

  interface Props {
    studioActiveCompId?: string;
    isCompManageMode?: boolean;
    selectedCompIds?: string[];
    onEditComponent?: (c: Component) => void;
    onCloneComponent?: (c: Component) => void;
    onNewComponent?: (e?: MouseEvent) => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }

  let {
    studioActiveCompId = $bindable(''),
    isCompManageMode = $bindable(false),
    selectedCompIds = $bindable<string[]>([]),
    onEditComponent = () => {},
    onCloneComponent = () => {},
    onNewComponent = () => {},
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

  function toggleCategory(catKey: string) {
    collapsedCategories[catKey] = !collapsedCategories[catKey];
  }

  let selectedComp = $derived.by(() => {
    return ws.components.find((c: Component) => c.id === studioActiveCompId) || ws.components[0] || null;
  });

  let studioTmplCode = $state('');
  let isDirty = $state(false);
  let lastSavedAt = $state('已同步');
  let studioPreviewJSON = $state('// 实时渲染中...');
  let studioPreviewError = $state('');
  let editorRef = $state<{ insertAtCursor: (snippet: string) => void } | null>(null);

  // 母版标签统计与多选
  let selectedFilterTags = $state<string[]>([]);
  let tagCounts = $derived.by(() => {
    const counts: Record<string, number> = {};
    ws.components.forEach((c: Component) => {
      if (c.scope_tag) {
        c.scope_tag.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean).forEach((t: string) => {
          counts[t] = (counts[t] || 0) + 1;
        });
      }
    });
    return counts;
  });
  let availableTags = $derived.by(() => Object.keys(tagCounts));

  let testParams = $state<Record<string, string | number | boolean>>({});
  let previewTimer: ReturnType<typeof setTimeout> | null = null;
  let lastLoadedCompId = '';

  function extractAllVars(code: string): string[] {
    const matches = code.matchAll(/\{\{\s*\.([a-zA-Z0-9_.]+)(?:\s*\|[^}]+)?\s*\}\}/g);
    const unique: string[] = [];
    for (const m of matches) {
      if (!unique.includes(m[1])) unique.push(m[1]);
    }
    return unique;
  }

  let detectedVars = $derived.by(() => extractAllVars(studioTmplCode));

  $effect(() => {
    const curComp = selectedComp;
    if (curComp && curComp.id !== lastLoadedCompId) {
      lastLoadedCompId = curComp.id;
      untrack(() => {
        studioTmplCode = curComp.payload_tmpl;
        isDirty = false;
        lastSavedAt = curComp.updated_at ? new Date(curComp.updated_at).toLocaleTimeString() : '已同步';
        runPreviewDirect(curComp.payload_tmpl);
      });
    }
  });

  function triggerPreviewDebounced() {
    if (previewTimer) clearTimeout(previewTimer);
    previewTimer = setTimeout(() => {
      runPreviewDirect(studioTmplCode);
    }, 250);
  }

  async function runPreviewDirect(code: string) {
    if (!code.trim()) {
      studioPreviewJSON = '{\n}';
      studioPreviewError = '';
      return;
    }
    try {
      studioPreviewJSON = await api.previewComponent(code, JSON.stringify(testParams), '');
      studioPreviewError = '';
    } catch (e: unknown) {
      studioPreviewError = e instanceof Error ? e.message : '渲染错误';
    }
  }

  function handleCancelChanges() {
    if (!selectedComp) return;
    studioTmplCode = selectedComp.payload_tmpl;
    isDirty = false;
    runPreviewDirect(studioTmplCode);
    ws.showToast('已放弃未保存的更改', 'info');
  }

  async function saveComp() {
    if (!selectedComp) return;
    selectedComp.payload_tmpl = studioTmplCode;
    await api.saveComponent(selectedComp);
    await ws.loadComponents();
    await ws.loadProfiles();
    isDirty = false;
    lastSavedAt = new Date().toLocaleTimeString();
    ws.showToast('母版已保存并提交 Git ✓', 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }
</script>

<div class="flex flex-1 min-h-0 overflow-hidden relative">
  <!-- 左栏：母版分类手风琴列表 -->
  <aside class="flex w-72 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50/70 dark:border-zinc-800 dark:bg-zinc-950/40">
    <div class="flex h-14 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
      <div class="flex items-center gap-2">
        <Icon name="sliders" size={16} color="emerald" />
        <span class="text-sm font-bold uppercase tracking-wider text-zinc-600 dark:text-zinc-300">语法母版库 ({ws.components.length})</span>
      </div>
      <div class="flex items-center gap-1.5">
        <button
          title={isCompManageMode ? '退出管理' : '批量整理母版'}
          onclick={() => { isCompManageMode = !isCompManageMode; selectedCompIds = []; }}
          type="button"
          class="flex h-7 w-7 items-center justify-center rounded-lg transition cursor-pointer shadow-2xs {isCompManageMode ? 'bg-amber-100 text-amber-700 border border-amber-300 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-800' : 'bg-white border border-zinc-200 text-zinc-500 hover:bg-zinc-100 dark:bg-zinc-900 dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800'}"
        >
          <Icon name="sliders" size={14} />
        </button>
        <button
          title="新建语法母版"
          onclick={onNewComponent}
          type="button"
          class="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-50 border border-blue-200 text-blue-600 hover:bg-blue-100 shadow-2xs dark:bg-blue-950/50 dark:border-blue-900/50 dark:text-blue-400 cursor-pointer transition"
        >
          <Icon name="plus" size={15} />
        </button>
      </div>
    </div>

    <!-- 标签多选筛选 -->
    <TagFilter {availableTags} {tagCounts} bind:selectedTags={selectedFilterTags} />

    <!-- 现代手风琴卡片分组 -->
    <div class="flex-1 overflow-y-auto p-3 space-y-4">
      {#each CATEGORIES as cat (cat.key)}
        {@const catComponents = ws.components.filter((c: Component) => c.category === cat.key && (selectedFilterTags.length === 0 || selectedFilterTags.some((t: string) => (c.scope_tag || '').includes(t))))}
        {@const isCollapsed = collapsedCategories[cat.key] === true}
        
        <div class="rounded-2xl border border-zinc-200/70 bg-zinc-100/60 p-1.5 dark:border-zinc-800/80 dark:bg-zinc-900/50">
          <button
            onclick={() => toggleCategory(cat.key)}
            type="button"
            class="w-full flex items-center justify-between px-2.5 py-2 rounded-xl hover:bg-zinc-200/50 dark:hover:bg-zinc-800/60 transition cursor-pointer select-none text-left"
          >
            <div class="flex items-center gap-2">
              <span class="text-zinc-400 text-[10px] transform transition-transform duration-200 {isCollapsed ? '' : 'rotate-90'}">▶</span>
              <Icon name={cat.icon} size={14} color={cat.color} />
              <span class="text-xs font-bold text-zinc-800 dark:text-zinc-200">{cat.title}</span>
            </div>
            <span class="text-[10px] font-mono px-2 py-0.5 rounded-full font-bold {catComponents.length > 0 ? 'bg-zinc-200 dark:bg-zinc-800 text-zinc-700 dark:text-zinc-300' : 'bg-transparent text-zinc-400'}">
              {catComponents.length}
            </span>
          </button>

          {#if !isCollapsed}
            <div class="mt-1 flex flex-col gap-1.5 px-1.5 pb-1.5 animate-in fade-in duration-150">
              {#each catComponents as comp (comp.id)}
                {@const isSelected = selectedComp?.id === comp.id}
                <div class="group relative flex items-center justify-between rounded-xl px-3 py-2.5 text-xs transition border shadow-2xs {isSelected && !isCompManageMode ? 'border-emerald-500/50 bg-white text-zinc-900 font-extrabold dark:border-emerald-500/40 dark:bg-zinc-900 dark:text-white' : 'border-transparent text-zinc-600 hover:bg-white/60 dark:text-zinc-400 dark:hover:bg-zinc-850'}">
                  {#if isSelected && !isCompManageMode}
                    <span class="absolute left-0 top-2 bottom-2 w-1.5 rounded-r-full bg-emerald-500"></span>
                  {/if}

                  <div class="flex items-center gap-2.5 truncate flex-1 pl-1.5">
                    {#if isCompManageMode}
                      <input
                        type="checkbox"
                        checked={selectedCompIds.includes(comp.id)}
                        onchange={(e) => {
                          if ((e.target as HTMLInputElement).checked) selectedCompIds = [...selectedCompIds, comp.id];
                          else selectedCompIds = selectedCompIds.filter((id: string) => id !== comp.id);
                        }}
                        class="rounded h-3.5 w-3.5 cursor-pointer"
                      />
                    {/if}
                    <button onclick={() => { studioActiveCompId = comp.id; }} type="button" class="truncate text-left flex-1 cursor-pointer font-medium">
                      {comp.name}
                    </button>
                  </div>

                  <div class="flex items-center gap-1.5 shrink-0">
                    {#if !isCompManageMode}
                      <span class="font-mono text-[9px] text-zinc-500 dark:text-zinc-400 font-bold px-1.5 py-0.5 rounded-md border border-zinc-200 dark:border-zinc-700 bg-zinc-100 dark:bg-zinc-800">
                        {comp.scope_tag}
                      </span>
                    {:else}
                      <button title="克隆母版" onclick={() => onCloneComponent(comp)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition">
                        <Icon name="copy" size={14} />
                      </button>
                      <button title="编辑元属性" onclick={() => onEditComponent(comp)} type="button" class="text-zinc-400 hover:text-blue-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition">
                        <Icon name="pencil" size={14} />
                      </button>
                      <button title="删除母版" onclick={() => triggerConfirm('确认删除母版？', '将永久删除母版 [' + comp.name + ']。所有引用此母版的成品积木也会被删除。', () => ws.batchDeleteComponents([comp.id]))} type="button" class="text-zinc-400 hover:text-rose-600 cursor-pointer p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition">
                        <Icon name="trash" size={14} />
                      </button>
                    {/if}
                  </div>
                </div>
              {:else}
                <span class="text-xs text-zinc-400 italic px-2 py-2 block text-center">暂无母版</span>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </aside>

  <!-- 右侧母版代码编辑 -->
  {#if selectedComp}
    <main class="flex flex-1 min-w-0 overflow-hidden bg-white dark:bg-zinc-950">
      <div class="flex flex-1 flex-col border-r border-zinc-200 dark:border-zinc-800">
        <div class="flex h-14 shrink-0 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
          <div class="flex items-center gap-2">
            <span class="text-sm font-extrabold text-zinc-900 dark:text-zinc-100">{selectedComp.name}</span>
            <span class="rounded-md border border-zinc-200 bg-zinc-100 px-2 py-0.5 font-mono text-xs font-bold text-zinc-500 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-400 shadow-2xs">
              [{selectedComp.mount_path}]
            </span>
          </div>

          <div class="flex items-center gap-2">
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
              onclick={saveComp}
              type="button"
              class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-bold text-white shadow-sm hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5"
            >
              <Icon name="check" size={14} />
              <span>{isDirty ? '保存母版 *' : '保存母版'}</span>
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-hidden p-4">
          <div class="h-full rounded-2xl border border-zinc-200 bg-white p-1.5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950">
            <CodeEditor
              bind:this={editorRef}
              value={studioTmplCode}
              dark={ws.isDark}
              onChange={(newVal: string) => {
                studioTmplCode = newVal;
                isDirty = true;
                triggerPreviewDebounced();
              }}
            />
          </div>
        </div>

        <div class="border-t border-zinc-200 bg-zinc-50/70 p-5 dark:border-zinc-800 dark:bg-zinc-950/70 space-y-3">
          <div class="flex items-center justify-between">
            <span class="text-sm font-bold text-zinc-800 dark:text-zinc-200">自动侦测参数桩 (Mock Inputs):</span>
            <span class="text-xs text-zinc-500 font-mono">检测到 {detectedVars.length} 个模板插孔</span>
          </div>

          {#if detectedVars.length === 0}
            <div class="text-xs text-zinc-400 italic py-2">当前模板无动态变量，纯静态输出。</div>
          {:else}
            <div class="grid grid-cols-2 xl:grid-cols-4 gap-4 max-h-40 overflow-y-auto pr-2">
              {#each detectedVars as v (v)}
                <label class="block">
                  <span class="text-[11px] font-mono font-bold text-zinc-600 dark:text-zinc-400 block mb-1 truncate" title={v}>.{v}</span>
                  <input
                    value={testParams[v] ?? ''}
                    oninput={(e) => {
                      const val = (e.target as HTMLInputElement).value;
                      let parsed: string | number | boolean = val;
                      if (val.trim() !== '' && !isNaN(Number(val))) parsed = Number(val);
                      else if (val.toLowerCase() === 'true') parsed = true;
                      else if (val.toLowerCase() === 'false') parsed = false;
                      testParams[v] = parsed;
                      triggerPreviewDebounced();
                    }}
                    placeholder="填入测试值"
                    class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500 transition shadow-2xs"
                  />
                </label>
              {/each}
            </div>
          {/if}
        </div>

        <div class="h-8 border-t border-zinc-200 bg-zinc-50 px-5 flex items-center justify-between text-xs font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
          <div class="flex items-center gap-4">
            <span class="flex items-center gap-2">
              <span class="h-2 w-2 rounded-full {isDirty ? 'bg-amber-400 shadow-[0_0_6px_#fbbf24]' : 'bg-emerald-500'}"></span>
              <span class="font-bold">{isDirty ? '未保存修改 (Ctrl+S)' : '已保存同步'}</span>
            </span>
            <span>保存于: {lastSavedAt}</span>
          </div>
          <div class="flex items-center gap-4">
            <span>行数: {studioTmplCode.split('\n').length} 行</span>
            <span>插孔: {detectedVars.length} 个</span>
          </div>
        </div>
      </div>

      <!-- 右侧渲染沙箱 -->
      <div class="flex w-[45%] flex-col bg-zinc-50/50 dark:bg-zinc-950/80">
        <div class="flex h-14 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
          <span class="font-mono text-sm font-bold text-zinc-500 dark:text-zinc-400">RENDERED JSON SANDBOX</span>
          {#if studioPreviewError}
            <span class="text-[11px] text-rose-600 font-mono font-bold flex items-center gap-1.5 bg-rose-50 dark:bg-rose-950/40 px-2.5 py-1 rounded-lg border border-rose-200 dark:border-rose-900/50 shadow-2xs">
              <Icon name="close" size={13} />
              <span>语法错误</span>
            </span>
          {:else}
            <span class="text-[11px] text-emerald-600 font-bold flex items-center gap-1.5 bg-emerald-50 dark:bg-emerald-950/40 px-2.5 py-1 rounded-lg border border-emerald-200 dark:border-emerald-900/50 shadow-2xs">
              <Icon name="check" size={13} />
              <span>合法 JSON</span>
            </span>
          {/if}
        </div>
        
        {#if studioPreviewError}
          <div class="m-4 p-4 rounded-xl bg-rose-50/80 text-rose-800 border border-rose-200/80 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-900/60 text-xs font-mono whitespace-pre-wrap overflow-y-auto max-h-48 shadow-inner transition-all">
            <div class="flex items-center gap-1.5 mb-2 font-bold text-rose-700 dark:text-rose-400">
              <Icon name="close" size={14} />
              <span>JSON 模板渲染或校验失败:</span>
            </div>
            {studioPreviewError}
          </div>
        {/if}

        <div class="flex-1 overflow-hidden p-4 pt-1">
          <div class="h-full rounded-2xl border border-zinc-200 bg-white p-1.5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950">
            <CodeEditor value={studioPreviewJSON} readonly={true} dark={ws.isDark} />
          </div>
        </div>

        <div class="h-8 border-t border-zinc-200 bg-zinc-50 px-5 flex items-center justify-between text-xs font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
          <span>挂载目标: {selectedComp.mount_path}</span>
          <span>大小: {(new Blob([studioPreviewJSON]).size / 1024).toFixed(1)} KB</span>
        </div>
      </div>
    </main>
  {:else}
    <div class="flex-1 flex items-center justify-center text-zinc-400 text-sm font-medium">
      暂无语法母版，请点击左上角「+」新建母版
    </div>
  {/if}
</div>