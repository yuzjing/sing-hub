<!-- frontend/src/lib/components/TagFilter.svelte -->
<script lang="ts">
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    availableTags?: string[];
    tagCounts?: Record<string, number>;
    selectedTags?: string[];
    borderless?: boolean;
    onFilterChange?: (tags: string[]) => void;
  }

  let {
    availableTags = [],
    tagCounts = {},
    selectedTags = $bindable([]),
    borderless = false,
    onFilterChange = () => {}
  }: Props = $props();

  let isOpen = $state(false);

  function toggleTag(tag: string) {
    if (selectedTags.includes(tag)) {
      selectedTags = selectedTags.filter((t: string) => t !== tag);
    } else {
      selectedTags = [...selectedTags, tag];
    }
    onFilterChange(selectedTags);
  }

  function clearAll() {
    selectedTags = [];
    onFilterChange(selectedTags);
  }

  
</script>

<div class="relative {borderless ? '' : 'border-b border-zinc-200 dark:border-zinc-800'} px-4 py-3">
  <div class="flex items-center justify-between gap-2">
    <button
      onclick={() => isOpen = !isOpen}
      type="button"
      class="w-full flex items-center justify-between rounded-lg border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3.5 py-2 text-xs font-medium text-zinc-700 dark:text-zinc-200 shadow-2xs hover:bg-zinc-50 dark:hover:bg-zinc-800 transition cursor-pointer"
    >
      <div class="flex items-center gap-2">
        <Icon name="sliders" size={14} color="default" />
        <span class="text-zinc-500 font-normal">标签:</span>
        <span class="font-bold {selectedTags.length > 0 ? 'text-blue-600 dark:text-blue-400' : 'text-zinc-700 dark:text-zinc-300'}">
          {selectedTags.length === 0 ? '全部显示' : `已选 ${selectedTags.length} 个`}
        </span>
      </div>
      <span class="text-[10px] text-zinc-400 font-mono">▾</span>
    </button>
  </div>

  {#if isOpen}
    <button
      type="button"
      tabindex="-1"
      aria-label="关闭筛选菜单"
      class="fixed inset-0 z-30 bg-transparent cursor-default border-none p-0 w-full h-full"
      onclick={() => isOpen = false}
    ></button>

    <div class="absolute left-4 right-4 top-14 z-40 rounded-2xl border border-zinc-200 bg-white p-3 shadow-2xl dark:border-zinc-700 dark:bg-zinc-900 space-y-2 text-sm max-h-64 overflow-y-auto animate-in fade-in duration-100">
      {#if availableTags.length === 0}
        <span class="text-xs text-zinc-500 block px-2 py-3 italic text-center">暂无可用标签</span>
      {:else}
        {#each availableTags as t (t)}
          <label class="flex items-center justify-between gap-2 px-3 py-2 rounded-xl hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer font-mono text-xs select-none transition">
            <div class="flex items-center gap-2.5">
              <input
                type="checkbox"
                checked={selectedTags.includes(t)}
                onchange={() => toggleTag(t)}
                class="rounded cursor-pointer h-3.5 w-3.5"
              />
              <span class="truncate">{t}</span>
            </div>
            {#if tagCounts[t]}
              <span class="text-[10px] font-mono px-2 py-0.5 rounded-full bg-zinc-200/70 dark:bg-zinc-700/60 text-zinc-700 dark:text-zinc-300 font-bold">
                {tagCounts[t]}
              </span>
            {/if}
          </label>
        {/each}
        <div class="border-t border-zinc-100 dark:border-zinc-800 pt-3 mt-1 flex justify-between items-center text-xs text-zinc-500 px-1">
          <button onclick={clearAll} type="button" class="hover:underline cursor-pointer hover:text-zinc-800 dark:hover:text-zinc-200 transition">清除全部</button>
          <button onclick={() => isOpen = false} type="button" class="font-bold text-blue-600 dark:text-blue-400 hover:text-blue-700 transition cursor-pointer">完成</button>
        </div>
      {/if}
    </div>
  {/if}
</div>