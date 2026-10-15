<!-- frontend/src/lib/components/ModuleModal.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Module, Component } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  let {
    open = false,
    editModule = null,
    onClose = () => {},
    onSave = () => {}
  } = $props<{
    open?: boolean;
    editModule?: Module | null;
    onClose?: () => void;
    onSave?: (m: Module) => void;
  }>();

  let id = $state('');
  let name = $state('');
  let templateId = $state('');
  let category = $state('io');
  let description = $state('');
  let paramValues = $state<Record<string, string | number | boolean>>({});

  let selectedTmpl = $derived.by(() => {
    return ws.components.find((c: Component) => c.id === templateId) || null;
  });

  let templateVars = $derived.by(() => {
    if (!selectedTmpl) return [];
    const matches = selectedTmpl.payload_tmpl.matchAll(/\{\{\s*\.([a-zA-Z0-9_.]+)(?:\s*\|[^}]+)?\s*\}\}/g);
    const unique: string[] = [];
    for (const m of matches) {
      if (!unique.includes(m[1])) unique.push(m[1]);
    }
    return unique;
  });

  $effect(() => {
    if (open) {
      if (editModule) {
        id = editModule.id;
        name = editModule.name;
        templateId = editModule.template_id;
        category = editModule.category;
        description = editModule.description;
        try {
          paramValues = JSON.parse(editModule.params_json || '{}');
        } catch {
          paramValues = {};
        }
      } else {
        id = '';
        name = '';
        templateId = ws.components[0]?.id || '';
        category = ws.components[0]?.category || 'io';
        description = '';
        paramValues = {};
      }
    }
  });

  function handleSave() {
    if (!name.trim()) {
      ws.showToast('请输入成品积木名称', 'error');
      return;
    }
    if (!templateId) {
      ws.showToast('请选择引用的语法母版', 'error');
      return;
    }

    let finalId = id.trim();
    if (finalId && !/^[a-zA-Z0-9_-]+$/.test(finalId)) {
      ws.showToast('ID 仅支持英文、数字、下划线与中划线', 'error');
      return;
    }
    if (!finalId) {
      finalId = 'mod-' + Date.now().toString(36);
    }

    const mod: Module = {
      id: finalId,
      template_id: templateId,
      name: name.trim(),
      category: category,
      params_json: JSON.stringify(paramValues),
      description: description.trim(),
    };
    onSave(mod);
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4">
    <div class="w-full max-w-lg rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800">
        <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
          <Icon name="boxes" color="blue" size={17} />
          <span>{editModule ? '编辑成品积木' : '创建新成品积木'}</span>
        </h3>
        <button onclick={onClose} class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={15} />
        </button>
      </div>

      <div class="space-y-3 text-xs">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">成品积木 ID (唯一标识)</span>
            <input
              bind:value={id}
              disabled={!!editModule}
              placeholder="如 out-hk-cn2 (留空自动生成)"
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
            />
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">成品名称</span>
            <input
              bind:value={name}
              placeholder="例如: 香港 CN2 高速出站"
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">引用的母版 (软链接指针)</span>
            <select
              bind:value={templateId}
              onchange={() => { if (selectedTmpl) category = selectedTmpl.category; }}
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 cursor-pointer"
            >
              {#each ws.components as c (c.id)}
                <option value={c.id}>{c.name} ({c.mount_path})</option>
              {/each}
            </select>
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">分类</span>
            <select
              bind:value={category}
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 cursor-pointer"
            >
              <option value="io">流量出入 (IO)</option>
              <option value="routing">路由决策 (Routing)</option>
              <option value="dns">域名解析 (DNS)</option>
              <option value="system">系统基础 (System)</option>
            </select>
          </label>
        </div>

        <div class="border-t border-zinc-100 pt-3 dark:border-zinc-800 space-y-2">
          <div class="flex items-center justify-between">
            <span class="font-bold text-zinc-700 dark:text-zinc-300">配置该成品的实参 (母版插孔):</span>
            <span class="font-mono text-[10px] text-zinc-400">检测到 {templateVars.length} 个插孔</span>
          </div>

          {#if templateVars.length === 0}
            <div class="text-[11px] text-zinc-400 italic">该母版无需外部变量，为纯静态结构。</div>
          {:else}
            <div class="grid grid-cols-2 gap-2.5 max-h-48 overflow-y-auto p-1">
              {#each templateVars as v (v)}
                <label class="block">
                  <span class="text-[10px] font-mono text-zinc-500 block mb-0.5 truncate" title={v}>.{v}</span>
                  <input
                    value={paramValues[v] ?? ''}
                    oninput={(e) => {
                      const val = (e.target as HTMLInputElement).value;
                      let parsed: string | number | boolean = val;
                      if (val.trim() !== '' && !isNaN(Number(val))) parsed = Number(val);
                      else if (val.toLowerCase() === 'true') parsed = true;
                      else if (val.toLowerCase() === 'false') parsed = false;
                      paramValues[v] = parsed;
                    }}
                    placeholder={v.startsWith('_') ? '留空使用环境上下文' : '填入具体实参'}
                    class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  />
                </label>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800">
        <button onclick={onClose} class="rounded-lg border border-zinc-300 px-3.5 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 cursor-pointer">取消</button>
        <button onclick={handleSave} class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer">保存成品</button>
      </div>
    </div>
  </div>
{/if}