<!-- frontend/src/lib/components/ComponentModal.svelte -->
<script lang="ts">
  import type { Strategy, Component } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';
  import { ws } from '$lib/workspace.svelte';

  interface Props {
    open?: boolean;
    editComponent?: Component | null;
    onClose?: () => void;
    onSave?: (comp: Component) => void;
  }

  let {
    open = false,
    editComponent = null,
    onClose = () => {},
    onSave = () => {}
  }: Props = $props();

  let id = $state('');
  let name = $state('');
  let category = $state('routing');
  let scopeTag = $state('universal');
  let mountPath = $state('route.rules');
  let strategy = $state<Strategy>('append');
  let payloadTmpl = $state('{\n  \n}');
  let description = $state('');

  $effect(() => {
    if (open) {
      if (editComponent) {
        id = editComponent.id;
        name = editComponent.name;
        category = editComponent.category;
        scopeTag = editComponent.scope_tag;
        mountPath = editComponent.mount_path;
        strategy = editComponent.strategy;
        payloadTmpl = editComponent.payload_tmpl;
        description = editComponent.description;
      } else {
        id = '';
        name = '';
        category = 'routing';
        scopeTag = 'universal';
        mountPath = 'route.rules';
        strategy = 'append';
        payloadTmpl = '{\n  \n}';
        description = '';
      }
    }
  });

  function handleSubmit() {
    if (!name.trim()) {
      ws.showToast('请输入母版名称', 'error');
      return;
    }
    if (!mountPath.trim()) {
      ws.showToast('请输入挂载路径', 'error');
      return;
    }

    let finalId = id.trim();
    if (finalId && !/^[a-zA-Z0-9_-]+$/.test(finalId)) {
      ws.showToast('ID 仅支持英文、数字、下划线与中划线', 'error');
      return;
    }
    if (!finalId) {
      finalId = 'comp-' + Date.now().toString(36);
    }

    onSave({
      id: finalId,
      name: name.trim(),
      category,
      scope_tag: scopeTag.trim(),
      mount_path: mountPath.trim(),
      strategy,
      payload_tmpl: payloadTmpl.trim(),
      description: description.trim(),
    });
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4">
    <div class="w-full max-w-lg rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800">
        <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
          <Icon name="sliders" color="emerald" size={17} />
          <span>{editComponent ? '编辑语法母版元数据' : '新建语法母版 (Template)'}</span>
        </h3>
        <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={15} />
        </button>
      </div>

      <div class="space-y-3 text-xs">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">母版唯一 ID</span>
            <input
              bind:value={id}
              disabled={!!editComponent}
              placeholder="如 tmpl-vless-outbound"
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:opacity-50"
            />
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">显示名称</span>
            <input
              bind:value={name}
              placeholder="如: VLESS 出站通用模板"
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>
        </div>

        <div class="grid grid-cols-3 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">分类</span>
            <select bind:value={category} class="w-full rounded-lg border border-zinc-300 bg-white px-2 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 cursor-pointer">
              <option value="io">流量出入 (IO)</option>
              <option value="routing">路由决策 (Routing)</option>
              <option value="dns">域名解析 (DNS)</option>
              <option value="system">系统基础 (System)</option>
            </select>
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">挂载路径</span>
            <input
              bind:value={mountPath}
              placeholder="如: outbounds"
              class="w-full rounded-lg border border-zinc-300 bg-white px-2 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">组装策略</span>
            <select bind:value={strategy} class="w-full rounded-lg border border-zinc-300 bg-white px-2 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 cursor-pointer">
              <option value="append">Append (数组追加)</option>
              <option value="merge">Merge (对象合并)</option>
            </select>
          </label>
        </div>
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800">
        <button onclick={onClose} type="button" class="rounded-lg border border-zinc-300 px-3.5 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 cursor-pointer">取消</button>
        <button onclick={handleSubmit} type="button" class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer">保存母版元数据</button>
      </div>
    </div>
  </div>
{/if}