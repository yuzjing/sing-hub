<!-- frontend/src/lib/components/ParamBinder.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Device } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    templateVars?: string[];
    templateCode?: string;
    params?: Record<string, string | number | boolean>;
    onChange?: () => void;
  }

  let {
    templateVars = [],
    templateCode = '',
    params = $bindable<Record<string, string | number | boolean>>({}),
    onChange = () => {}
  }: Props = $props();

  let searchFilter = $state('');
  let activeMenuVar = $state<string | null>(null);
  let menuPos = $state({ top: 0, right: 0 });

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
      menuPos = { top: btn.bottom + 6, right: window.innerWidth - btn.right };
    }
  }

  function applyRef(varKey: string, refExpr: string) {
    params[varKey] = refExpr;
    activeMenuVar = null;
    onChange();
  }

  function handleInputChange(varKey: string, rawVal: string) {
    let parsed: string | number | boolean = rawVal;
    if (rawVal.trim() !== '' && !isNaN(Number(rawVal))) parsed = Number(rawVal);
    else if (rawVal.toLowerCase() === 'true') parsed = true;
    else if (rawVal.toLowerCase() === 'false') parsed = false;
    params[varKey] = parsed;
    onChange();
  }
</script>

<div class="space-y-4">
  {#if templateVars.length > 4}
    <div class="flex items-center justify-between gap-3 border-b border-zinc-100 pb-3 dark:border-zinc-800">
      <span class="text-xs text-zinc-500 font-mono">共 {templateVars.length} 个参数插孔</span>
      <input
        bind:value={searchFilter}
        placeholder="搜索插孔或字段名..."
        class="rounded-lg border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-500 w-56 transition shadow-2xs"
      />
    </div>
  {/if}

  {#if filteredVars.length === 0}
    <div class="text-sm text-zinc-500 italic py-6 text-center">无匹配的参数插孔。</div>
  {:else}
    <div class="grid grid-cols-1 xl:grid-cols-2 gap-4 max-h-104 overflow-y-auto pr-2">
      {#each filteredVars as v (v)}
        {@const curVal = String(params[v] ?? '')}
        {@const isRef = curVal.startsWith('@node:')}
        {@const targetField = fieldMap[v]}

        <div class="rounded-xl border border-zinc-200/80 bg-white p-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-950 space-y-2.5 transition-all hover:border-blue-300 dark:hover:border-blue-800">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 truncate pr-2">
              {#if targetField}
                <span class="rounded bg-blue-50 dark:bg-blue-950/60 text-blue-700 dark:text-blue-300 font-mono text-xs font-bold px-2 py-0.5 shrink-0 border border-blue-100 dark:border-blue-900/50">
                  "{targetField}"
                </span>
                <span class="text-zinc-300 dark:text-zinc-700 text-xs">←</span>
              {/if}
              <span class="text-xs font-mono text-zinc-500 dark:text-zinc-400 truncate" title={v}>.{v}</span>
            </div>

            <button
              onclick={(e) => toggleMenu(v, e)}
              type="button"
              class="flex items-center gap-1.5 rounded-lg px-2 py-1 text-[11px] font-medium {isRef ? 'bg-blue-50 text-blue-600 dark:bg-blue-950/60 dark:text-blue-400 font-bold border border-blue-200 dark:border-blue-900/60' : 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300 hover:bg-zinc-200 dark:hover:bg-zinc-700 border border-transparent'} transition cursor-pointer shadow-2xs shrink-0"
            >
              <span>{isRef ? '🔗 已引用节点' : '⚡ 引用节点'}</span>
              <span class="text-[9px] opacity-70">▾</span>
            </button>
          </div>

          <div class="relative">
            <input
              value={curVal}
              oninput={(e) => handleInputChange(v, (e.target as HTMLInputElement).value)}
              placeholder={targetField ? `为 "${targetField}" 填入实参或引用` : '填入具体实参'}
              class="w-full rounded-lg border {isRef ? 'border-blue-400/80 bg-blue-50/20 text-blue-700 dark:border-blue-800 dark:text-blue-300' : 'border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-950 text-zinc-900 dark:text-zinc-100'} px-3 py-1.5 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-blue-500 transition shadow-2xs"
            />
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

{#if activeMenuVar}
  {@const targetVar = activeMenuVar}
  <button type="button" tabindex="-1" aria-label="关闭引用菜单" class="fixed inset-0 z-50 bg-transparent cursor-default border-none p-0 w-full h-full" onclick={() => activeMenuVar = null}></button>

  <div style="top: {menuPos.top}px; right: {menuPos.right}px;" class="fixed z-50 w-80 rounded-2xl border border-zinc-200 bg-white/95 p-3 shadow-2xl backdrop-blur-xl dark:border-zinc-700 dark:bg-zinc-900/95 text-sm space-y-3 animate-in fade-in zoom-in-95 duration-100 max-h-104 overflow-y-auto">
    <div>
      <span class="text-xs font-bold text-zinc-500 uppercase tracking-wider block px-1.5 mb-2 items-center gap-1.5">
        <Icon name="server" size={14} color="blue" />
        <span>选择网络节点的端点或属性</span>
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