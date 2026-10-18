<!-- frontend/src/lib/components/ModuleMetaModal.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Module, Component } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    open?: boolean;
    editModule?: Module | null;
    onClose?: () => void;
    onSave?: (mod: Module) => void;
  }

  let {
    open = false,
    editModule = null,
    onClose = () => {},
    onSave = () => {}
  }: Props = $props();

  let id = $state('');
  let name = $state('');
  let templateId = $state('');
  let category = $state('io');
  let tagsStr = $state('');

  $effect(() => {
    if (open) {
      if (editModule) {
        id = editModule.id;
        name = editModule.name;
        templateId = editModule.template_id;
        category = editModule.category;
        tagsStr = (editModule.tags || []).join(', ');
      } else {
        id = '';
        name = '';
        templateId = ws.components[0]?.id || '';
        category = ws.components[0]?.category || 'io';
        tagsStr = '';
      }
    }
  });

  function handleSubmit() {
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
      tags: tagsStr.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean),
      params_json: editModule?.params_json || '{}',
      description: editModule?.description || '',
    };
    onSave(mod);
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4 animate-in fade-in duration-150">
    <div class="w-full max-w-md rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800">
        <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
          <Icon name="boxes" color="blue" size={17} />
          <span>{editModule ? '编辑积木基础属性' : '新建成品积木'}</span>
        </h3>
        <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={15} />
        </button>
      </div>

      <div class="space-y-3.5 text-xs">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">成品积木 ID</span>
            <input
              bind:value={id}
              disabled={!!editModule}
              placeholder="如 out-hk-cn2"
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
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">引用的母版</span>
            <select
              bind:value={templateId}
              onchange={() => {
                const c = ws.components.find((comp: Component) => comp.id === templateId);
                if (c) category = c.category;
              }}
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 cursor-pointer"
            >
              {#each ws.components as c (c.id)}
                <option value={c.id}>{c.name} ({c.mount_path})</option>
              {/each}
            </select>
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">业务分类</span>
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

        <!-- 关键补齐：积木标签输入 -->
        <label class="block">
          <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">积木标签 (逗号分隔)</span>
          <input
            bind:value={tagsStr}
            placeholder="例如: hk, cn2, 4k, proxy"
            class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </label>
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800">
        <button onclick={onClose} type="button" class="rounded-lg border border-zinc-300 px-3.5 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 cursor-pointer">
          取消
        </button>
        <button onclick={handleSubmit} type="button" class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer">
          保存属性
        </button>
      </div>
    </div>
  </div>
{/if}