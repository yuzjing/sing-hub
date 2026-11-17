<!-- frontend/src/lib/components/ModuleParamsModal.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Module, Component } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    open?: boolean;
    module?: Module | null;
    onClose?: () => void;
    onSave?: (mod: Module) => void | Promise<void>;
  }
  let saving = $state(false);

  let {
    open = false,
    module = null,
    onClose = () => {},
    onSave = () => {}
  }: Props = $props();

  let paramValues = $state<Record<string, unknown>>({});

  let targetTmpl = $derived.by(() => {
    if (!module) return null;
    return ws.components.find((c: Component) => c.id === module.template_id) || null;
  });

  let templateVars = $derived.by(() => {
    if (!targetTmpl) return [];
    const matches = targetTmpl.payload_tmpl.matchAll(/\{\{\s*\.([a-zA-Z0-9_.]+)(?:\s*\|[^}]+)?\s*\}\}/g);
    const unique: string[] = [];
    for (const m of matches) {
      if (!unique.includes(m[1])) unique.push(m[1]);
    }
    return unique;
  });

  $effect(() => {
    if (open && module) {
      try {
        paramValues = JSON.parse(module.params_json || '{}');
      } catch {
        paramValues = {};
      }
    }
  });

  async function handleSave() {
  if (saving || !module) return;
  saving = true;
  try {
    await onSave({ ...module, params_json: JSON.stringify(paramValues) });
  } finally {
    saving = false;
  }
}
</script>

{#if open && module}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4 animate-in fade-in duration-150">
    <div class="w-full max-w-lg rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-4">
      <div class="flex items-start justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800">
        <div>
          <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
            <Icon name="sliders" color="blue" size={17} />
            <span>调整实参: {module.name}</span>
          </h3>
          <span class="text-[11px] text-zinc-400 font-mono mt-0.5 block">
            母版: {targetTmpl?.name || module.template_id} [{targetTmpl?.mount_path || 'path'}]
          </span>
        </div>
        <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={15} />
        </button>
      </div>

      <div class="space-y-3">
        {#if templateVars.length === 0}
          <div class="py-8 text-center text-xs text-zinc-400 italic">
            该积木引用的母版无任何动态插孔，为纯静态结构。
          </div>
        {:else}
          <div class="grid grid-cols-2 gap-3 max-h-60 overflow-y-auto p-1">
            {#each templateVars as v (v)}
              <label class="block">
                <span class="text-[10px] font-mono text-zinc-500 block mb-0.5 truncate" title={v}>.{v}</span>
                <input
                  value={typeof paramValues[v] === 'object' && paramValues[v] !== null ? JSON.stringify(paramValues[v]) : (paramValues[v] ?? '')}
                  oninput={(e) => {
                    const val = (e.target as HTMLInputElement).value;
                    const trimmed = val.trim();
                    let parsed: unknown = val;
                    if ((trimmed.startsWith('[') && trimmed.endsWith(']')) || (trimmed.startsWith('{') && trimmed.endsWith('}'))) {
                      try {
                        parsed = JSON.parse(trimmed);
                      } catch {
                        parsed = val;
                      }
                    } else if (trimmed !== '' && !isNaN(Number(trimmed))) {
                      parsed = Number(trimmed);
                    } else if (trimmed.toLowerCase() === 'true') {
                      parsed = true;
                    } else if (trimmed.toLowerCase() === 'false') {
                      parsed = false;
                    }
                    paramValues[v] = parsed;
                  }}
                  placeholder={v.startsWith('_') ? '系统默认上下文' : '填入具体实参'}
                  class="w-full rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
              </label>
            {/each}
          </div>
        {/if}
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800">
        <button onclick={onClose} disabled={saving} type="button" class="rounded-lg border border-zinc-300 px-3.5 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 cursor-pointer disabled:opacity-50">
          取消
        </button>
        <button onclick={handleSave} disabled={saving} type="button" class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer disabled:opacity-60 disabled:cursor-not-allowed">
          {saving ? '应用中...' : '应用实参'}
        </button>
      </div>
    </div>
  </div>
{/if}