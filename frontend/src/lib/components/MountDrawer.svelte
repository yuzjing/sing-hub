<!-- frontend/src/lib/components/MountDrawer.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Component } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  let { open = false, onClose }: { open: boolean; onClose: () => void } = $props();
  let searchQuery = $state('');
  let selectedToMount = $state<string[]>([]);
  let selectedFilterTags = $state<string[]>([]);
  let isTagDropdownOpen = $state(false);

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

  const CATEGORIES = [
    { key: 'io', title: '流量出入 (IO)', color: 'blue' as const },
    { key: 'routing', title: '路由决策 (Routing)', color: 'amber' as const },
    { key: 'dns', title: '域名解析 (DNS)', color: 'emerald' as const },
    { key: 'system', title: '系统基础设施 (System)', color: 'purple' as const }
  ];

  function isComponentMounted(compId: string): boolean {
    if (!ws.activeProfile || !ws.activeProfile.components) return false;
    return ws.activeProfile.components.some(b => b.component_id === compId);
  }

  async function handleSingleMount(c: Component) {
    if (isComponentMounted(c.id)) return;
    await ws.attachComponents([c.id]);
    ws.showToast(`已装载 [${c.name}] ✓`, 'success');
  }

  async function handleApply() {
    if (selectedToMount.length === 0) return;
    const count = selectedToMount.length;
    await ws.attachComponents(selectedToMount);
    selectedToMount = [];
    ws.showToast(`已应用装载 ${count} 个积木 ✓`, 'success');
  }

  async function handleConfirmAndClose() {
    if (selectedToMount.length > 0) {
      const count = selectedToMount.length;
      await ws.attachComponents(selectedToMount);
      selectedToMount = [];
      ws.showToast(`已装载 ${count} 个积木 ✓`, 'success');
    }
    onClose();
  }

  function toggleTagFilter(tag: string) {
    if (selectedFilterTags.includes(tag)) {
      selectedFilterTags = selectedFilterTags.filter(t => t !== tag);
    } else {
      selectedFilterTags = [...selectedFilterTags, tag];
    }
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm">
    <div class="flex w-130 flex-col bg-white shadow-2xl dark:bg-zinc-950 border-l border-zinc-200 dark:border-zinc-800">
      <div class="flex h-14 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
        <div>
          <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
            <Icon name="boxes" color="blue" size={17} />
            <span>挑选积木挂载到方案</span>
          </h3>
          <span class="text-[11px] text-zinc-400">正在配给: <strong class="text-zinc-700 dark:text-zinc-200">{ws.activeProfile?.name}</strong></span>
        </div>
        <button onclick={onClose} class="text-zinc-400 hover:text-zinc-700 dark:hover:text-white cursor-pointer p-1">
          <Icon name="close" size={16} />
        </button>
      </div>

      <div class="p-3.5 border-b border-zinc-200 dark:border-zinc-800 flex gap-2 relative">
        <input
          bind:value={searchQuery}
          placeholder="搜索名称、挂载点..."
          class="flex-1 rounded-lg border border-zinc-300 bg-zinc-50 px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-900 focus:outline-none"
        />
        <button
          onclick={() => (isTagDropdownOpen = !isTagDropdownOpen)}
          class="rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs text-zinc-600 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1 shrink-0"
        >
          <Icon name="sliders" size={13} />
          <span>{selectedFilterTags.length === 0 ? '标签筛选' : `${selectedFilterTags.length} 个标签`}</span>
        </button>

        {#if isTagDropdownOpen}
          <div class="absolute right-3.5 top-12 z-30 w-52 rounded-xl border border-zinc-200 bg-white p-2 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-1 text-xs">
            {#if availableTags.length === 0}
              <span class="text-[10px] text-zinc-400 block p-1">暂无可用标签</span>
            {:else}
              {#each availableTags as t (t)}
                <label class="flex items-center gap-2 px-1.5 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer font-mono text-[11px]">
                  <input type="checkbox" checked={selectedFilterTags.includes(t)} onchange={() => toggleTagFilter(t)} class="rounded" />
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

      <div class="flex-1 overflow-y-auto p-4 space-y-4">
        {#each CATEGORIES as cat (cat.key)}
          <div>
            <!-- 修复了 block flex 冲突 -->
            <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider flex items-center gap-1.5 mb-2">
              <span class="h-1.5 w-1.5 rounded-full {cat.color === 'blue' ? 'bg-blue-500' : cat.color === 'amber' ? 'bg-amber-500' : cat.color === 'emerald' ? 'bg-emerald-500' : 'bg-purple-500'}"></span>
              <span>{cat.title}</span>
            </span>
            <div class="space-y-2">
              {#each ws.components.filter(c => c.category === cat.key && (searchQuery === '' || c.name.toLowerCase().includes(searchQuery.toLowerCase())) && (selectedFilterTags.length === 0 || selectedFilterTags.some(t => (c.scope_tag || '').includes(t)))) as comp (comp.id)}
                {@const mounted = isComponentMounted(comp.id)}
                <div class="flex items-center justify-between rounded-xl border border-zinc-200 p-3.5 dark:border-zinc-800 transition {mounted ? 'bg-zinc-100/60 opacity-60 dark:bg-zinc-900/30' : 'bg-zinc-50/50 hover:border-zinc-300 dark:bg-zinc-900/50 dark:hover:border-zinc-700'}">
                  <div class="flex items-center gap-2.5 truncate flex-1 mr-2">
                    {#if !mounted}
                      <input
                        type="checkbox"
                        checked={selectedToMount.includes(comp.id)}
                        onchange={(e) => {
                          if ((e.target as HTMLInputElement).checked) selectedToMount = [...selectedToMount, comp.id];
                          else selectedToMount = selectedToMount.filter(id => id !== comp.id);
                        }}
                        class="rounded"
                      />
                    {/if}
                    <div class="truncate">
                      <h4 class="text-xs font-semibold text-zinc-900 dark:text-zinc-100 truncate">{comp.name}</h4>
                      <span class="font-mono text-[10px] text-zinc-400">{comp.mount_path} [{comp.scope_tag}]</span>
                    </div>
                  </div>

                  {#if mounted}
                    <span class="rounded bg-zinc-200/80 px-2.5 py-1 text-[11px] font-mono text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                      已在方案中 ✓
                    </span>
                  {:else}
                    <button
                      onclick={() => handleSingleMount(comp)}
                      class="rounded-lg bg-blue-600 px-3 py-1 text-xs font-semibold text-white hover:bg-blue-500 shadow-sm transition cursor-pointer flex items-center gap-1"
                    >
                      <Icon name="plus" size={12} />
                      <span>挂载</span>
                    </button>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        {/each}
      </div>

      <div class="flex items-center justify-between border-t border-zinc-200 bg-white p-3.5 dark:border-zinc-800 dark:bg-zinc-950">
        <span class="text-xs text-zinc-400">
          {#if selectedToMount.length > 0}
            已选定 <strong>{selectedToMount.length}</strong> 个新积木
          {:else}
            勾选积木进行批量操作
          {/if}
        </span>
        <div class="flex items-center gap-2">
          <button
            disabled={selectedToMount.length === 0}
            onclick={handleApply}
            class="rounded-lg border border-zinc-300 bg-zinc-50 px-3 py-1.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer disabled:opacity-40 transition"
          >
            应用 (不退出)
          </button>
          <button
            onclick={handleConfirmAndClose}
            class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-sm transition"
          >
            {selectedToMount.length > 0 ? `确认装载 (${selectedToMount.length})` : '完成'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}