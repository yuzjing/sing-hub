<!-- frontend/src/lib/components/MountDrawer.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Module } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    open?: boolean;
    onClose?: () => void;
  }

  let { open = false, onClose = () => {} }: Props = $props();

  let searchText = $state('');
  let selectedCategory = $state('all');
  let selectedTags = $state<string[]>([]);
  let tagMode = $state<'and' | 'or'>('and');
  let onlyUnmounted = $state(false);
  let mountingId = $state('');
  let isBatchMounting = $state(false);

  const CATEGORIES = [
    { key: 'all', title: '全部' },
    { key: 'io', title: '流量出入 (IO)' },
    { key: 'routing', title: '路由决策 (Routing)' },
    { key: 'dns', title: '域名解析 (DNS)' },
    { key: 'system', title: '系统基础 (System)' }
  ];

  let mountedModuleIds = $derived.by(() => {
    return new Set((ws.activeProfile?.modules || []).map((b: { module_id: string }) => b.module_id));
  });

  let availableTags = $derived.by(() => {
    const tagSet = new Set<string>();
    ws.modules.forEach((m: Module) => {
      if (selectedCategory === 'all' || m.category === selectedCategory) {
        (m.tags || []).forEach((t: string) => tagSet.add(t));
      }
    });
    return Array.from(tagSet).sort();
  });

  let filteredModules = $derived.by(() => {
    const q = searchText.trim().toLowerCase();
    return ws.modules.filter((m: Module) => {
      const matchCat = selectedCategory === 'all' || m.category === selectedCategory;
      const modTags = m.tags || [];
      const matchTags = selectedTags.length === 0 || (
        tagMode === 'and'
          ? selectedTags.every((t: string) => modTags.includes(t))
          : selectedTags.some((t: string) => modTags.includes(t))
      );
      const matchMounted = !onlyUnmounted || !mountedModuleIds.has(m.id);
      const matchText = !q ||
        m.name.toLowerCase().includes(q) ||
        m.id.toLowerCase().includes(q) ||
        (m.template?.mount_path || '').toLowerCase().includes(q) ||
        modTags.some((t: string) => t.toLowerCase().includes(q));
      return matchCat && matchTags && matchMounted && matchText;
    });
  });

  let unmountedFilteredIds = $derived.by(() => {
    return filteredModules
      .filter((m: Module) => !mountedModuleIds.has(m.id))
      .map((m: Module) => m.id);
  });

  function toggleTag(tag: string) {
    if (selectedTags.includes(tag)) {
      selectedTags = selectedTags.filter((t: string) => t !== tag);
    } else {
      selectedTags = [...selectedTags, tag];
    }
  }

  async function handleMount(m: Module) {
    if (mountingId || isBatchMounting || mountedModuleIds.has(m.id)) return;
    mountingId = m.id;
    try {
      await ws.attachModules([m.id]);
      ws.showToast('已装配成品积木 [' + m.name + '] ✓', 'success');
    } finally {
      mountingId = '';
    }
  }

  async function handleBatchMount() {
    if (isBatchMounting || unmountedFilteredIds.length === 0) return;
    isBatchMounting = true;
    try {
      const count = unmountedFilteredIds.length;
      await ws.attachModules(unmountedFilteredIds);
      ws.showToast(`已批量装配 ${count} 个积木 ✓`, 'success');
    } finally {
      isBatchMounting = false;
    }
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
          <span class="text-xs text-zinc-400">正在配给方案: <strong>{ws.activeProfile?.name || '-'}</strong></span>
        </div>
        <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={16} />
        </button>
      </div>

      <div class="p-4 border-b border-zinc-200 dark:border-zinc-800 space-y-3">
        <input
          bind:value={searchText}
          placeholder="搜索名称、ID、挂载路径或标签..."
          class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />

        <div class="flex flex-wrap gap-1.5">
          {#each CATEGORIES as cat (cat.key)}
            <button
              onclick={() => selectedCategory = cat.key}
              type="button"
              class="px-2.5 py-1 rounded-md text-xs font-medium transition cursor-pointer {selectedCategory === cat.key ? 'bg-blue-600 text-white' : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 hover:bg-zinc-200'}"
            >
              {cat.title}
            </button>
          {/each}
        </div>

        {#if availableTags.length > 0}
          <div class="rounded-xl border border-zinc-200/80 dark:border-zinc-800 bg-zinc-50/70 dark:bg-zinc-950/60 p-2.5 space-y-2">
            <div class="flex items-center justify-between text-xs">
              <span class="font-semibold text-zinc-500">标签组合筛选</span>
              <div class="flex items-center gap-2">
                <button
                  onclick={() => tagMode = tagMode === 'and' ? 'or' : 'and'}
                  type="button"
                  class="rounded px-2 py-0.5 font-mono font-bold border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 text-zinc-700 dark:text-zinc-300 cursor-pointer"
                  title="切换多标签组合逻辑"
                >
                  {tagMode === 'and' ? 'AND (全部满足)' : 'OR (满足任一)'}
                </button>
                {#if selectedTags.length > 0}
                  <button onclick={() => selectedTags = []} type="button" class="text-zinc-400 hover:text-zinc-700 dark:hover:text-zinc-200 cursor-pointer">
                    清空
                  </button>
                {/if}
              </div>
            </div>

            <div class="flex flex-wrap gap-1.5 max-h-24 overflow-y-auto">
              {#each availableTags as tag (tag)}
                {@const active = selectedTags.includes(tag)}
                <button
                  onclick={() => toggleTag(tag)}
                  type="button"
                  class="px-2 py-0.5 rounded-md text-xs font-mono border transition cursor-pointer {active ? 'bg-emerald-600 border-emerald-600 text-white font-bold' : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-700 text-zinc-600 dark:text-zinc-400 hover:border-emerald-400'}"
                >
                  #{tag}
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <div class="flex items-center justify-between text-xs">
          <label class="flex items-center gap-1.5 text-zinc-600 dark:text-zinc-400 cursor-pointer select-none">
            <input type="checkbox" bind:checked={onlyUnmounted} class="rounded cursor-pointer" />
            <span>仅显示未装配 ({unmountedFilteredIds.length})</span>
          </label>

          {#if unmountedFilteredIds.length > 0}
            <button
              disabled={isBatchMounting}
              onclick={handleBatchMount}
              type="button"
              class="rounded-lg bg-emerald-600 px-3 py-1 text-xs font-semibold text-white hover:bg-emerald-500 cursor-pointer shadow-xs transition disabled:opacity-60"
            >
              {isBatchMounting ? '装配中...' : `一键装配筛选 (${unmountedFilteredIds.length})`}
            </button>
          {/if}
        </div>
      </div>

      <div class="flex-1 overflow-y-auto p-4 space-y-2">
        {#if filteredModules.length === 0}
          <div class="text-center py-12 text-zinc-400 text-xs">
            暂无匹配的成品积木。
          </div>
        {:else}
          {#each filteredModules as mod (mod.id)}
            {@const isMounted = mountedModuleIds.has(mod.id)}
            <div class="flex items-center justify-between p-3 rounded-xl border border-zinc-200 dark:border-zinc-800 bg-zinc-50/50 dark:bg-zinc-950/50 hover:border-zinc-300 dark:hover:border-zinc-700 transition gap-2">
              <div class="space-y-1 min-w-0 flex-1">
                <span class="text-xs font-bold text-zinc-800 dark:text-zinc-200 block truncate">{mod.name}</span>
                <div class="flex items-center flex-wrap gap-1.5 text-xs text-zinc-400 font-mono">
                  <span class="truncate">{mod.template?.name || mod.template_id}</span>
                  {#if mod.template?.mount_path}
                    <span class="rounded bg-zinc-200/60 dark:bg-zinc-800 px-1.5 py-0.5">[{mod.template.mount_path}]</span>
                  {/if}
                  {#each (mod.tags || []) as t (t)}
                    <span class="text-blue-600 dark:text-blue-400">#{t}</span>
                  {/each}
                </div>
              </div>

              {#if isMounted}
                <span class="px-2.5 py-1 text-xs text-zinc-400 font-medium bg-zinc-100 dark:bg-zinc-800 rounded-lg shrink-0">
                  已在方案中 ✓
                </span>
              {:else}
                <button
                  disabled={mountingId === mod.id || isBatchMounting}
                  onclick={() => handleMount(mod)}
                  type="button"
                  class="rounded-lg bg-blue-600 px-3 py-1 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-sm transition flex items-center gap-1 shrink-0 disabled:opacity-60"
                >
                  <Icon name="plus" size={13} />
                  <span>{mountingId === mod.id ? '装配中...' : '装配'}</span>
                </button>
              {/if}
            </div>
          {/each}
        {/if}
      </div>

      <div class="border-t border-zinc-200 p-3 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 flex justify-end">
        <button onclick={onClose} type="button" class="rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-4 py-1.5 text-xs text-zinc-700 dark:text-zinc-300 cursor-pointer">
          完成
        </button>
      </div>

    </div>
  </div>
{/if}