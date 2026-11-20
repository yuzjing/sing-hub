<!-- frontend/src/lib/components/ParamBinder.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import Icon from '$lib/components/Icon.svelte';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import type { Device } from '$lib/types';

  interface Props {
    templateVars?: string[];
    templateCode?: string;
    params?: Record<string, unknown>;
    onChange?: () => void;
  }

  let {
    templateVars = [],
    templateCode = '',
    params = $bindable<Record<string, unknown>>({}),
    onChange = () => {}
  }: Props = $props();

  let searchFilter = $state('');
  let activeMenuVar = $state<string | null>(null);
  let menuStyle = $state('');

  // 高级长文本 / 复合 JSON 编辑沙箱弹窗状态
  let isModalOpen = $state(false);
  let activeModalVar = $state('');
  let modalDraftText = $state('');
  let modalFormatError = $state('');
  let modalEditorRef = $state<{ insertAtCursor: (snippet: string) => void } | null>(null);

  let fieldMap = $derived.by(() => {
    const mapping: Record<string, string> = {};
    if (!templateCode) return mapping;
    const regex = /"([^"\\]*(?:\\.[^"\\]*)*)"\s*:\s*"?\{\{\s*\.([a-zA-Z0-9_.]+)/g;
    for (const match of templateCode.matchAll(regex)) {
      mapping[match[2]] = match[1];
    }
    return mapping;
  });

  let filteredVars = $derived.by(() => {
    if (!searchFilter.trim()) return templateVars;
    return templateVars.filter((v: string) => v.toLowerCase().includes(searchFilter.toLowerCase()));
  });

  function toggleMenu(varKey: string, event: MouseEvent) {
    if (activeMenuVar === varKey) {
      activeMenuVar = null;
    } else {
      activeMenuVar = varKey;
      const btn = (event.currentTarget as HTMLElement).getBoundingClientRect();
      const right = Math.max(12, window.innerWidth - btn.right);
      const spaceBelow = window.innerHeight - btn.bottom - 16;
      const spaceAbove = btn.top - 16;

      if (spaceBelow < 260 && spaceAbove > spaceBelow) {
        const maxH = Math.min(416, Math.max(160, spaceAbove));
        const bottom = window.innerHeight - btn.top + 6;
        menuStyle = `bottom: ${bottom}px; right: ${right}px; max-height: ${maxH}px;`;
      } else {
        const maxH = Math.min(416, Math.max(160, spaceBelow));
        const top = btn.bottom + 6;
        menuStyle = `top: ${top}px; right: ${right}px; max-height: ${maxH}px;`;
      }
    }
  }

  function applyRef(varKey: string, refExpr: string) {
    params[varKey] = refExpr;
    activeMenuVar = null;
    onChange();
  }

  function handleInputChange(varKey: string, rawVal: string) {
    const trimmed = rawVal.trim();
    let parsed: unknown = rawVal;
    if ((trimmed.startsWith('[') && trimmed.endsWith(']')) || (trimmed.startsWith('{') && trimmed.endsWith('}'))) {
      try {
        parsed = JSON.parse(trimmed);
      } catch {
        parsed = rawVal;
      }
    } else if (trimmed !== '' && !isNaN(Number(trimmed))) {
      parsed = Number(trimmed);
    } else if (trimmed.toLowerCase() === 'true') {
      parsed = true;
    } else if (trimmed.toLowerCase() === 'false') {
      parsed = false;
    }
    params[varKey] = parsed;
    onChange();
  }

  // 打开全功能代码沙箱弹窗
  function openCodeModal(v: string) {
    activeModalVar = v;
    modalFormatError = '';
    const raw = params[v];
    if (typeof raw === 'object' && raw !== null) {
      modalDraftText = JSON.stringify(raw, null, 2);
    } else {
      const s = String(raw ?? '');
      // 尝试美化现存的单行 JSON
      const trimmed = s.trim();
      if ((trimmed.startsWith('[') && trimmed.endsWith(']')) || (trimmed.startsWith('{') && trimmed.endsWith('}'))) {
        try {
          modalDraftText = JSON.stringify(JSON.parse(trimmed), null, 2);
        } catch {
          modalDraftText = s;
        }
      } else {
        modalDraftText = s;
      }
    }
    isModalOpen = true;
  }

  // 一键格式化与排版校验
  function formatModalJSON() {
    try {
      const parsed = JSON.parse(modalDraftText);
      modalDraftText = JSON.stringify(parsed, null, 2);
      modalFormatError = '';
    } catch (e: unknown) {
      modalFormatError = e instanceof Error ? e.message : 'JSON 格式有误，无法自动排版';
    }
  }

  // 将节点引用精准插入当前编辑器光标位置 (解决多条引用痛点)
  function insertRefAtCursor(refExpr: string) {
    if (modalEditorRef) {
      modalEditorRef.insertAtCursor(`"${refExpr}"`);
    } else {
      modalDraftText += `"${refExpr}"`;
    }
  }

  // 提交沙箱编辑
  function commitModal() {
    handleInputChange(activeModalVar, modalDraftText);
    isModalOpen = false;
  }
</script>

<div class="space-y-3.5">
  {#if templateVars.length > 4}
    <div class="flex items-center justify-between gap-3 border-b border-zinc-100 pb-2.5 dark:border-zinc-800">
      <span class="text-xs text-zinc-500 font-mono">共 {templateVars.length} 个参数插孔</span>
      <input
        bind:value={searchFilter}
        placeholder="搜索插孔..."
        class="rounded-lg border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-500 w-44 transition shadow-2xs"
      />
    </div>
  {/if}

  {#if filteredVars.length === 0}
    <div class="text-sm text-zinc-500 italic py-6 text-center">无匹配的参数插孔。</div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3.5 max-h-104 overflow-y-auto pr-1">
      {#each filteredVars as v (v)}
        {@const rawParam = params[v]}
        {@const curVal = typeof rawParam === 'object' && rawParam !== null ? JSON.stringify(rawParam) : String(rawParam ?? '')}
        {@const isRef = typeof rawParam === 'string' && rawParam.startsWith('@node:')}
        {@const isComplex = typeof rawParam === 'object' || curVal.startsWith('[') || curVal.startsWith('{') || curVal.length > 40}
        {@const targetField = fieldMap[v]}

        <div class="rounded-xl border border-zinc-200/80 bg-white p-3 shadow-xs dark:border-zinc-800 dark:bg-zinc-950/80 space-y-2 transition-all hover:border-blue-300 dark:hover:border-blue-800">
          <div class="flex items-center justify-between gap-2">
            <!-- 字段标签 -->
            <div class="flex items-center gap-1.5 flex-wrap min-w-0 flex-1">
              {#if targetField}
                <span class="rounded bg-blue-50 dark:bg-blue-950/60 text-blue-700 dark:text-blue-300 font-mono text-[11px] font-bold px-1.5 py-0.5 shrink-0 border border-blue-100 dark:border-blue-900/50">
                  "{targetField}"
                </span>
                <span class="text-zinc-300 dark:text-zinc-700 text-[10px] shrink-0">←</span>
              {/if}
              <span class="text-xs font-mono font-bold text-zinc-700 dark:text-zinc-300 truncate" title={v}>.{v}</span>
            </div>

            <!-- 右侧操作栏：新增展开编辑沙箱 ⤢ 按钮 + 闪电快速引用 ⚡ -->
            <div class="flex items-center gap-1 shrink-0">
              <button
                onclick={() => openCodeModal(v)}
                type="button"
                title="打开全功能代码编辑沙箱 (适合长文本、数组、复杂 JSON 及多重节点引用)"
                class="flex h-6.5 px-1.5 items-center justify-center gap-1 rounded-lg border border-zinc-200 bg-zinc-50 text-[11px] font-mono text-zinc-600 hover:bg-blue-50 hover:text-blue-600 dark:border-zinc-700 dark:bg-zinc-800/80 dark:text-zinc-400 dark:hover:bg-blue-950/40 dark:hover:text-blue-300 transition cursor-pointer shadow-2xs"
              >
                <span>⤢</span>
                {#if isComplex}
                  <span class="text-[9px] font-bold text-blue-600 dark:text-blue-400">JSON</span>
                {/if}
              </button>

              <button
                onclick={(e) => toggleMenu(v, e)}
                type="button"
                title={isRef ? '已绑定节点属性，点击切换' : '点击单值引用设备节点属性 (@node:...)'}
                class="flex h-6.5 w-6.5 items-center justify-center rounded-lg transition cursor-pointer shrink-0 shadow-2xs {isRef ? 'bg-blue-50 text-blue-600 border border-blue-200 dark:bg-blue-950/60 dark:text-blue-400 dark:border-blue-800' : 'bg-zinc-100 text-zinc-500 hover:text-amber-600 hover:bg-amber-50 dark:bg-zinc-800/80 dark:text-zinc-400 dark:hover:bg-amber-950/40 dark:hover:text-amber-300 border border-transparent'}"
              >
                <span class="text-xs leading-none">{isRef ? '🔗' : '⚡'}</span>
              </button>
            </div>
          </div>

          <!-- 单行输入框 (若超长可随时点击 ⤢ 进入弹窗) -->
          <div class="relative">
            <input
              value={curVal}
              oninput={(e) => handleInputChange(v, (e.target as HTMLInputElement).value)}
              placeholder={isComplex ? '复杂结构，建议点击右上角 ⤢ 展开编辑' : ''}
              class="w-full rounded-lg border {isRef ? 'border-blue-400/80 bg-blue-50/20 text-blue-700 dark:border-blue-800 dark:text-blue-300 font-bold' : isComplex ? 'border-zinc-300 dark:border-zinc-700 bg-zinc-50/40 dark:bg-zinc-900/40 font-mono text-blue-600 dark:text-blue-400' : 'border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-950 text-zinc-900 dark:text-zinc-100'} px-3 py-1.5 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-500 transition shadow-2xs"
            />
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- ============================================================================== -->
<!-- 1. 单值快速覆盖引用菜单 (原 ⚡ 浮层)                                           -->
<!-- ============================================================================== -->
{#if activeMenuVar}
  {@const targetVar = activeMenuVar}
  <button type="button" tabindex="-1" aria-label="关闭引用菜单" class="fixed inset-0 z-50 bg-transparent cursor-default border-none p-0 w-full h-full" onclick={() => activeMenuVar = null}></button>

  <div style={menuStyle} class="fixed z-50 w-80 rounded-2xl border border-zinc-200 bg-white/95 p-3 shadow-2xl backdrop-blur-xl dark:border-zinc-700 dark:bg-zinc-900/95 text-sm space-y-3 animate-in fade-in zoom-in-95 duration-100 overflow-y-auto">
    <div>
      <span class="text-xs font-bold text-zinc-500 uppercase tracking-wider block px-1.5 mb-2 items-center gap-1.5">
        <Icon name="server" size={14} color="blue" />
        <span>一键覆盖单值引用 (复杂结构请用 ⤢)</span>
      </span>

      {#if ws.devices.length === 0}
        <span class="text-xs text-zinc-400 italic px-2 block py-2">暂无接入节点</span>
      {:else}
        <div class="space-y-2">
          {#each ws.devices as dev (dev.id)}
            <div class="p-2.5 rounded-xl bg-zinc-50 dark:bg-zinc-800/50 border border-zinc-100 dark:border-zinc-800">
              <span class="text-sm font-bold text-zinc-800 dark:text-zinc-100 block truncate mb-1.5">
                {dev.name} <span class="text-xs font-mono text-zinc-400 font-normal">({dev.id})</span>
              </span>
              
              <div class="space-y-1.5 font-mono text-xs">
                {#if dev.props && dev.props.length > 0}
                  {#each dev.props as p (p.key)}
                    <button onclick={() => applyRef(targetVar, '@node:' + dev.id + '.' + p.key)} type="button" class="w-full text-left px-2.5 py-1.5 rounded-lg bg-white dark:bg-zinc-900 border border-zinc-200/60 dark:border-zinc-700/60 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-950/40 flex items-center justify-between cursor-pointer transition shadow-2xs">
                      <span class="font-semibold truncate">{p.label || p.key}</span>
                      <span class="opacity-70 truncate max-w-36 text-zinc-500">{p.value}</span>
                    </button>
                  {/each}
                {:else}
                  {#each dev.endpoints as ep, i (i)}
                    <button onclick={() => applyRef(targetVar, '@node:' + dev.id + '.endpoints[' + i + ']')} type="button" class="w-full text-left px-2.5 py-1.5 rounded-lg bg-white dark:bg-zinc-900 border border-zinc-200/60 dark:border-zinc-700/60 text-blue-600 dark:text-blue-400 hover:bg-blue-50 flex items-center justify-between cursor-pointer transition shadow-2xs">
                      <span class="font-semibold">端点 {i + 1}</span>
                      <span class="opacity-70 truncate max-w-36 text-zinc-500">{ep}</span>
                    </button>
                  {/each}
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    {#if String(params[targetVar] ?? '').startsWith('@node:')}
      <div class="border-t border-zinc-200 pt-2 dark:border-zinc-800">
        <button onclick={() => { params[targetVar] = ''; activeMenuVar = null; onChange(); }} type="button" class="w-full text-center px-3 py-1.5 rounded-lg bg-rose-50 text-rose-600 hover:bg-rose-100 dark:bg-rose-950/40 dark:text-rose-400 text-xs font-semibold cursor-pointer transition">
          ✕ 解除节点引用，切回手动填值
        </button>
      </div>
    {/if}
  </div>
{/if}

<!-- ============================================================================== -->
<!-- 2. 全功能复合参数代码编辑沙箱 (支持光标位置精确插入 @node 属性)                 -->
<!-- ============================================================================== -->
{#if isModalOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
    <div class="w-full max-w-3xl rounded-2xl border border-zinc-200 bg-white p-5 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 flex flex-col space-y-3.5 max-h-[90vh]">
      
      <!-- 头部：标题与格式化操作 -->
      <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800 shrink-0">
        <div class="flex items-center gap-2">
          <Icon name="sliders" color="blue" size={17} />
          <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100">
            高级参数编辑: <span class="font-mono text-blue-600 dark:text-blue-400">.{activeModalVar}</span>
          </h3>
          {#if fieldMap[activeModalVar]}
            <span class="text-xs text-zinc-400 font-mono">(目标字段: "{fieldMap[activeModalVar]}")</span>
          {/if}
        </div>

        <div class="flex items-center gap-2">
          <button
            onclick={formatModalJSON}
            type="button"
            class="rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-800 px-3 py-1 text-xs font-medium text-zinc-700 dark:text-zinc-200 hover:bg-zinc-50 dark:hover:bg-zinc-700/60 cursor-pointer shadow-2xs transition flex items-center gap-1.5"
          >
            <span>✨ 格式化 JSON</span>
          </button>
          <button onclick={() => isModalOpen = false} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
            <Icon name="close" size={15} />
          </button>
        </div>
      </div>

      {#if modalFormatError}
        <div class="px-3 py-1.5 rounded-lg bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-300 text-xs font-mono border border-rose-200 dark:border-rose-900/40 shrink-0">
          ⚠️ {modalFormatError}
        </div>
      {/if}

      <!-- 中部主编辑区：直接调用现成的 CodeEditor -->
      <div class="h-72 w-full rounded-xl border border-zinc-200 dark:border-zinc-800 overflow-hidden bg-white dark:bg-zinc-950 shrink-0">
        <CodeEditor
          bind:this={modalEditorRef}
          value={modalDraftText}
          dark={ws.isDark}
          onChange={(newVal: string) => {
            modalDraftText = newVal;
          }}
        />
      </div>

      <!-- 底部节点属性注入托盘：光标位置精确插入 (彻底解决多条引用痛点) -->
      <div class="rounded-xl border border-blue-200/60 bg-blue-50/30 p-3 dark:border-blue-900/40 dark:bg-blue-950/20 space-y-2 flex-1 min-h-0 flex flex-col">
        <div class="flex items-center justify-between shrink-0">
          <span class="text-xs font-bold text-blue-900 dark:text-blue-300 flex items-center gap-1">
            <Icon name="server" size={13} color="blue" />
            <span>点击向当前编辑器光标位置插入属性 (可连续插入多条):</span>
          </span>
          <span class="text-[11px] text-zinc-400 font-mono">自动添加双引号安全包裹</span>
        </div>

        {#if ws.devices.length === 0}
          <span class="text-xs text-zinc-400 italic">暂无可用节点</span>
        {:else}
          <div class="overflow-y-auto space-y-2 pr-1 max-h-32">
            {#each ws.devices as dev (dev.id)}
              <div class="flex items-center gap-1.5 flex-wrap text-xs font-mono">
                <span class="font-bold text-zinc-700 dark:text-zinc-300 shrink-0 bg-white dark:bg-zinc-900 px-1.5 py-0.5 rounded border border-zinc-200 dark:border-zinc-800">
                  {dev.name}:
                </span>
                {#if dev.props && dev.props.length > 0}
                  {#each dev.props as p (p.key)}
                    <button
                      onclick={() => insertRefAtCursor(`@node:${dev.id}.${p.key}`)}
                      type="button"
                      title={`插入 "@node:${dev.id}.${p.key}"`}
                      class="px-2 py-0.5 rounded-md bg-white dark:bg-zinc-900 border border-blue-200 dark:border-blue-800 text-blue-600 dark:text-blue-300 hover:bg-blue-600 hover:text-white transition cursor-pointer shadow-2xs truncate max-w-48"
                    >
                      +{p.label || p.key}
                    </button>
                  {/each}
                {:else}
                  {#each dev.endpoints as ep, i (i)}
                    <button
                      onclick={() => insertRefAtCursor(`@node:${dev.id}.endpoints[${i}]`)}
                      type="button"
                      class="px-2 py-0.5 rounded-md bg-white dark:bg-zinc-900 border border-blue-200 dark:border-blue-800 text-blue-600 dark:text-blue-300 hover:bg-blue-600 hover:text-white transition cursor-pointer shadow-2xs"
                    >
                      +端点{i + 1}
                    </button>
                  {/each}
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <!-- 底部控制按钮 -->
      <div class="flex justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800 shrink-0">
        <button
          onclick={() => isModalOpen = false}
          type="button"
          class="rounded-lg border border-zinc-300 px-4 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 cursor-pointer"
        >
          取消
        </button>
        <button
          onclick={commitModal}
          type="button"
          class="rounded-lg bg-blue-600 px-5 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-sm transition"
        >
          应用并回写
        </button>
      </div>

    </div>
  </div>
{/if}