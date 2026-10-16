<!-- frontend/src/lib/components/MountDrawer.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Module } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  let { open = false, onClose = () => {} } = $props<{ open?: boolean; onClose?: () => void }>();

  let searchText = $state('');
  let selectedCategory = $state<string>('all');

  const CATEGORIES = [
    { key: 'all', title: '全部' },
    { key: 'io', title: '流量出入 (IO)' },
    { key: 'routing', title: '路由决策 (Routing)' },
    { key: 'dns', title: '域名解析 (DNS)' },
    { key: 'system', title: '系统基础 (System)' }
  ];

  // 当前方案已经挂载的所有 module_id 集合
  let mountedModuleIds = $derived.by(() => {
    return new Set((ws.activeProfile?.modules || []).map(b => b.module_id));
  });

  let filteredModules = $derived.by(() => {
    return ws.modules.filter((m: Module) => {
      const matchCat = selectedCategory === 'all' || m.category === selectedCategory;
      const matchText = !searchText.trim() || 
        m.name.toLowerCase().includes(searchText.toLowerCase()) ||
        (m.template?.mount_path || '').toLowerCase().includes(searchText.toLowerCase());
      return matchCat && matchText;
    });
  });

  async function handleMount(m: Module) {
    if (mountedModuleIds.has(m.id)) return;
    await ws.attachModules([m.id]);
    ws.showToast(`已装配成品积木 [${m.name}] ✓`, 'success');
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-xs">
    <div class="w-full max-w-md bg-white dark:bg-zinc-900 h-full shadow-2xl flex flex-col border-l border-zinc-200 dark:border-zinc-800 animate-in slide-in-from-right duration-200">
      
      <div class="flex items-center justify-between border-b border-zinc-200 p-4 dark:border-zinc-800">
        <div>
          <h2 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
            <Icon name="boxes" color="emerald" size={17} />
            <span>挑选成品积木装配到方案</span>
          </h2>
          <span class="text-[11px] text-zinc-400">正在配给方案: <strong>{ws.activeProfile?.name || '-'}</strong></span>
        </div>
        <button onclick={onClose} class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={16} />
        </button>
      </div>

      <div class="p-4 border-b border-zinc-200 dark:border-zinc-800 space-y-2.5">
        <input
          bind:value={searchText}
          placeholder="搜索成品积木名称、挂载路径..."
          class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />

        <div class="flex flex-wrap gap-1.5">
          {#each CATEGORIES as cat (cat.key)}
            <button
              onclick={() => selectedCategory = cat.key}
              class="px-2 py-0.5 rounded-md text-[11px] font-medium transition cursor-pointer {selectedCategory === cat.key ? 'bg-blue-600 text-white' : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 hover:bg-zinc-200'}"
            >
              {cat.title}
            </button>
          {/each}
        </div>
      </div>

      <div class="flex-1 overflow-y-auto p-4 space-y-2">
        {#if filteredModules.length === 0}
          <div class="text-center py-12 text-zinc-400 text-xs">
            暂无匹配的成品积木。请前往「成品积木库」基于母版创建。
          </div>
        {:else}
          {#each filteredModules as mod (mod.id)}
            {@const isMounted = mountedModuleIds.has(mod.id)}
            <div class="flex items-center justify-between p-3 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-zinc-50/50 dark:bg-zinc-950/50 hover:border-zinc-300 dark:hover:border-zinc-700 transition">
              <div class="space-y-0.5">
                <span class="text-xs font-bold text-zinc-800 dark:text-zinc-200 block">{mod.name}</span>
                <div class="flex items-center gap-2 text-[10px] text-zinc-400 font-mono">
                  <span>母版: {mod.template?.name || mod.template_id}</span>
                  {#if mod.template?.mount_path}
                    <span class="rounded bg-zinc-200/60 dark:bg-zinc-800 px-1 py-0.2">[{mod.template.mount_path}]</span>
                  {/if}
                </div>
              </div>

              {#if isMounted}
                <span class="px-2.5 py-1 text-xs text-zinc-400 font-medium bg-zinc-100 dark:bg-zinc-800 rounded-lg">
                  已在方案中 ✓
                </span>
              {:else}
                <button
                  onclick={() => handleMount(mod)}
                  class="rounded-lg bg-blue-600 px-3 py-1 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-sm transition flex items-center gap-1"
                >
                  <Icon name="plus" size={13} />
                  <span>装配</span>
                </button>
              {/if}
            </div>
          {/each}
        {/if}
      </div>

      <div class="border-t border-zinc-200 p-3 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 flex justify-end">
        <button onclick={onClose} class="rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-4 py-1.5 text-xs text-zinc-700 dark:text-zinc-300 cursor-pointer">
          完成
        </button>
      </div>

    </div>
  </div>
{/if}