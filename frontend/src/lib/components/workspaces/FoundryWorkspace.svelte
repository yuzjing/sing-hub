<!-- frontend/src/lib/components/workspaces/FoundryWorkspace.svelte -->
<script lang="ts">
  import { untrack } from 'svelte';
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Component } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';

  let {
    studioActiveCompId = $bindable(''),
    isCompManageMode = $bindable(false),
    selectedCompIds = $bindable<string[]>([]),
    onEditComponent = () => {},
    onCloneComponent = () => {},
    onNewComponent = () => {},
    triggerConfirm = () => {}
  } = $props<{
    studioActiveCompId?: string;
    isCompManageMode?: boolean;
    selectedCompIds?: string[];
    onEditComponent?: (c: Component) => void;
    onCloneComponent?: (c: Component) => void;
    onNewComponent?: () => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }>();

  const CATEGORIES = [
    { key: 'io', title: '流量出入 (IO)' },
    { key: 'routing', title: '路由决策 (Routing)' },
    { key: 'dns', title: '域名解析 (DNS)' },
    { key: 'system', title: '系统基础设施 (System)' }
  ];

  let selectedComp = $derived.by(() => {
    return ws.components.find((c: Component) => c.id === studioActiveCompId) || ws.components[0] || null;
  });

  let studioTmplCode = $state('');
  let isDirty = $state(false);
  let lastSavedAt = $state('已同步');
  let studioPreviewJSON = $state('// 实时渲染中...');
  let studioPreviewError = $state('');
  let showContextVarMenu = $state(false);
  let editorRef = $state<{ insertAtCursor: (snippet: string) => void } | null>(null);

  let isTagDropdownOpen = $state(false);
  let selectedFilterTags = $state<string[]>([]);
  let testParams = $state<Record<string, string | number | boolean>>({});
  let previewTimer: ReturnType<typeof setTimeout> | null = null;
  let lastLoadedCompId = '';

  let availableTags = $derived.by(() => {
    const tags: string[] = [];
    ws.components.forEach((c: Component) => {
      if (c.scope_tag) {
        c.scope_tag.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean).forEach(t => {
          if (!tags.includes(t)) tags.push(t);
        });
      }
    });
    return tags;
  });

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

  function handleInsertVar(snippet: string) {
    if (editorRef) editorRef.insertAtCursor(snippet);
    else studioTmplCode += snippet;
    isDirty = true;
    showContextVarMenu = false;
    triggerPreviewDebounced();
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

  function toggleTagFilter(tag: string) {
    if (selectedFilterTags.includes(tag)) {
      selectedFilterTags = selectedFilterTags.filter((t: string) => t !== tag);
    } else {
      selectedFilterTags = [...selectedFilterTags, tag];
    }
  }
</script>

<div class="flex flex-1 min-h-0 overflow-hidden relative">
  <!-- 左侧：母版分类列表 -->
  <aside class="flex w-64 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50/70 dark:border-zinc-800 dark:bg-zinc-950/40">
    <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-3.5 dark:border-zinc-800">
      <span class="text-[11px] font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500">语法母版库 (TEMPLATES)</span>
      <div class="flex items-center gap-1">
        <button
          title={isCompManageMode ? '退出管理' : '批量整理母版'}
          onclick={() => { isCompManageMode = !isCompManageMode; selectedCompIds = []; }}
          class="flex h-6 w-6 items-center justify-center rounded text-xs transition cursor-pointer {isCompManageMode ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300 font-bold' : 'text-zinc-400 hover:bg-zinc-200/50 hover:text-zinc-700 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
        >
          <Icon name="sliders" size={14} />
        </button>
        <button
          title="新建语法母版"
          onclick={onNewComponent}
          class="flex h-6 w-6 items-center justify-center rounded text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/50 cursor-pointer"
        >
          <Icon name="plus" size={15} />
        </button>
      </div>
    </div>

    <!-- 标签筛选 -->
    <div class="relative border-b border-zinc-200 px-3 py-2 dark:border-zinc-800">
      <div class="flex items-center justify-between">
        <span class="text-[10px] font-mono text-zinc-400">标签筛选:</span>
        <button onclick={() => (isTagDropdownOpen = !isTagDropdownOpen)} class="rounded border border-zinc-200 bg-white px-2 py-0.5 text-[10px] font-mono text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer">
          {selectedFilterTags.length === 0 ? '全部显示' : `已选 ${selectedFilterTags.length} 个`} ▾
        </button>
      </div>

      {#if isTagDropdownOpen}
        <div class="absolute left-3 right-3 top-10 z-30 rounded-lg border border-zinc-200 bg-white p-2 shadow-xl dark:border-zinc-700 dark:bg-zinc-900 space-y-1 text-xs">
          {#if availableTags.length === 0}
            <span class="text-[10px] text-zinc-400 block px-1">暂无标签</span>
          {:else}
            {#each availableTags as t (t)}
              <label class="flex items-center gap-2 px-1.5 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer font-mono text-[11px]">
                <input type="checkbox" checked={selectedFilterTags.includes(t)} onchange={() => toggleTagFilter(t)} class="rounded cursor-pointer" />
                <span>{t}</span>
              </label>
            {/each}
            <div class="border-t border-zinc-100 pt-1 flex justify-between items-center text-[10px] text-zinc-400">
              <button onclick={() => (selectedFilterTags = [])} class="hover:underline">清除</button>
              <button onclick={() => (isTagDropdownOpen = false)} class="font-bold text-blue-500">完成</button>
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <div class="flex-1 overflow-y-auto p-2 space-y-4">
      {#each CATEGORIES as cat (cat.key)}
        <div>
          <span class="text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider px-2 block mb-1">
            {cat.title}
          </span>
          <div class="flex flex-col gap-1">
            {#each ws.components.filter((c: Component) => c.category === cat.key && (selectedFilterTags.length === 0 || selectedFilterTags.some((t: string) => (c.scope_tag || '').includes(t)))) as comp (comp.id)}
              <div class="group flex items-center justify-between rounded-lg px-2.5 py-2 text-xs transition {selectedComp?.id === comp.id && !isCompManageMode ? 'bg-white shadow-xs ring-1 ring-zinc-300 text-zinc-900 font-semibold dark:bg-zinc-800 dark:text-white' : 'text-zinc-600 hover:bg-zinc-200/50 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
                <div class="flex items-center gap-2 truncate flex-1">
                  {#if isCompManageMode}
                    <input
                      type="checkbox"
                      checked={selectedCompIds.includes(comp.id)}
                      onchange={(e) => {
                        if ((e.target as HTMLInputElement).checked) selectedCompIds = [...selectedCompIds, comp.id];
                        else selectedCompIds = selectedCompIds.filter((id: string) => id !== comp.id);
                      }}
                      class="rounded cursor-pointer"
                    />
                  {/if}
                  <button onclick={() => { studioActiveCompId = comp.id; }} class="truncate text-left flex-1 cursor-pointer">
                    {comp.name}
                  </button>
                </div>

                <div class="flex items-center gap-1">
                  {#if !isCompManageMode}
                    <span class="font-mono text-[9px] text-zinc-400">[{comp.scope_tag}]</span>
                  {:else}
                    <button title="克隆母版" onclick={() => onCloneComponent(comp)} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                      <Icon name="copy" size={13} />
                    </button>
                    <button title="编辑元属性" onclick={() => onEditComponent(comp)} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                      <Icon name="pencil" size={13} />
                    </button>
                    <button title="删除母版" onclick={() => triggerConfirm('确认删除母版？', `将永久删除母版 [${comp.name}]。所有引用此母版的成品积木也会被删除。`, () => ws.batchDeleteComponents([comp.id]))} class="text-zinc-400 hover:text-rose-500 cursor-pointer p-0.5">
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

  <!-- 右侧母版代码编辑与沙箱 -->
  {#if selectedComp}
    <main class="flex flex-1 min-w-0 overflow-hidden bg-white dark:bg-zinc-950">
      <div class="flex flex-1 flex-col border-r border-zinc-200 dark:border-zinc-800">
        <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
          <div class="flex items-center gap-2">
            <span class="font-mono text-xs font-semibold text-zinc-700 dark:text-zinc-300">{selectedComp.name}</span>
            <span class="rounded bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.2 font-mono text-[10px] text-zinc-400">[{selectedComp.mount_path}]</span>
          </div>

          <div class="flex items-center gap-2 relative">
            <button
              onclick={() => (showContextVarMenu = !showContextVarMenu)}
              class="rounded-md border border-zinc-300 bg-zinc-50 px-2.5 py-1 text-xs text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1 transition"
            >
              <Icon name="plus" size={12} />
              <span>插入变量 ▾</span>
            </button>

            {#if showContextVarMenu}
              <div class="absolute right-36 top-10 z-40 w-64 rounded-xl border border-zinc-200 bg-white p-2.5 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 text-xs space-y-2.5">
                <div>
                  <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1 mb-1">节点网络上下文 (NODE)</span>
                  <button onclick={() => handleInsertVar('{{ ._node.endpoint | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block">._node.endpoint (通信地址)</button>
                  <button onclick={() => handleInsertVar('{{ ._node.id | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block">._node.id (机器标识)</button>
                  <button onclick={() => handleInsertVar('{{ ._node.ssh_host | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block">._node.ssh_host (SSH地址)</button>
                </div>
                <div class="border-t border-zinc-100 pt-1.5 dark:border-zinc-800">
                  <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1 mb-1">网络与安全 (NETWORK/SEC)</span>
                  <button onclick={() => handleInsertVar('{{ ._net.easytier_cidr | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block text-emerald-600">._net.easytier_cidr (组网网段)</button>
                  <button onclick={() => handleInsertVar('{{ ._sec.uuid | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block text-rose-600">._sec.uuid (UUID)</button>
                </div>
              </div>
            {/if}

            <!-- 需求 1：提供放弃修改按钮 -->
            {#if isDirty}
              <button
                onclick={handleCancelChanges}
                class="rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-800 px-2.5 py-1 text-xs text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-zinc-700 cursor-pointer transition"
              >
                放弃修改
              </button>
            {/if}

            <button
              onclick={saveComp}
              class="rounded-md bg-blue-600 px-3.5 py-1 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-2xs transition flex items-center gap-1.5"
            >
              <Icon name="check" size={13} />
              <span>{isDirty ? '保存母版 *' : '保存母版'}</span>
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-hidden p-3">
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

        <div class="border-t border-zinc-200 bg-zinc-50/70 p-4 dark:border-zinc-800 dark:bg-zinc-950/70 space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300">自动侦测参数桩 (Mock Inputs):</span>
            <span class="text-[10px] text-zinc-400 font-mono">检测到 {detectedVars.length} 个模板插孔</span>
          </div>

          {#if detectedVars.length === 0}
            <div class="text-[11px] text-zinc-400 italic">当前模板无动态变量，纯静态输出。</div>
          {:else}
            <div class="grid grid-cols-2 md:grid-cols-4 gap-2.5">
              {#each detectedVars as v (v)}
                <label class="block">
                  <span class="text-[10px] font-mono text-zinc-500 block mb-0.5 truncate" title={v}>.{v}</span>
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
                    class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  />
                </label>
              {/each}
            </div>
          {/if}
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
            <span>行数: {studioTmplCode.split('\n').length} 行</span>
            <span>模板插孔: {detectedVars.length} 个</span>
          </div>
        </div>
      </div>

      <!-- 右侧渲染沙箱 -->
      <div class="flex w-[45%] flex-col bg-zinc-50/50 dark:bg-zinc-950/80">
        <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
          <span class="font-mono text-xs font-semibold text-zinc-500">RENDERED JSON SANDBOX</span>
          {#if studioPreviewError}
            <span class="text-[11px] text-rose-500 font-mono font-bold flex items-center gap-1" title={studioPreviewError}>
              <Icon name="close" size={12} />
              <span>语法错误</span>
            </span>
          {:else}
            <span class="text-[10px] text-emerald-500 font-semibold flex items-center gap-1">
              <Icon name="check" size={12} />
              <span>合法 JSON</span>
            </span>
          {/if}
        </div>

        {#if studioPreviewError}
          <div class="m-3 p-3 rounded-lg bg-rose-50 text-rose-800 border border-rose-200 dark:bg-rose-950/60 dark:text-rose-200 dark:border-rose-800 text-[11px] font-mono whitespace-pre-wrap overflow-y-auto max-h-48 shadow-inner">
            {studioPreviewError}
          </div>
        {/if}

        <div class="flex-1 overflow-hidden p-3 pt-0">
          <CodeEditor value={studioPreviewJSON} readonly={true} dark={ws.isDark} />
        </div>

        <div class="h-7 border-t border-zinc-200 bg-zinc-50 px-3 flex items-center justify-between text-[11px] font-mono text-zinc-400 dark:border-zinc-800 dark:bg-zinc-900/80">
          <span>挂载目标: {selectedComp.mount_path}</span>
          <span>大小: {(new Blob([studioPreviewJSON]).size / 1024).toFixed(1)} KB</span>
        </div>
      </div>
    </main>
  {:else}
    <div class="flex-1 flex items-center justify-center text-zinc-400 text-xs">
      暂无语法母版，请点击左上角「+」新建母版
    </div>
  {/if}
</div>