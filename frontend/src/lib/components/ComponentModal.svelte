<!-- frontend/src/lib/components/ComponentModal.svelte -->
<script lang="ts">
  let {
    open = false,
    onClose,
    onCreate,
  }: {
    open: boolean;
    onClose: () => void;
    onCreate: (comp: {
      name: string;
      category: string;
      scope_tag: string;
      mount_path: string;
      strategy: 'append' | 'merge';
      payload_tmpl: string;
    }) => void;
  } = $props();

  let name = $state('');
  let category = $state('routing');
  let scopeTag = $state('universal');
  let mountPath = $state('route.rules');
  let strategy = $state<'append' | 'merge'>('append');
  let payloadTmpl = $state('{\n  "outbound": "direct"\n}');

  const PRESET_CAPSULES = [
    { label: 'route.rules', path: 'route.rules', strategy: 'append', cat: 'routing' },
    { label: 'outbounds', path: 'outbounds', strategy: 'append', cat: 'io' },
    { label: 'inbounds', path: 'inbounds', strategy: 'append', cat: 'io' },
    { label: 'route.rule_set', path: 'route.rule_set', strategy: 'append', cat: 'routing' },
    { label: 'dns.servers', path: 'dns.servers', strategy: 'append', cat: 'dns' },
    { label: 'dns.rules', path: 'dns.rules', strategy: 'append', cat: 'dns' },
    { label: 'experimental', path: 'experimental', strategy: 'merge', cat: 'system' },
    { label: 'log', path: 'log', strategy: 'merge', cat: 'system' },
  ] as const;

  function selectCapsule(c: typeof PRESET_CAPSULES[number]) {
    mountPath = c.path;
    strategy = c.strategy as 'append' | 'merge';
    category = c.cat;
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
    <div class="w-120 rounded-xl border border-zinc-200 bg-white p-5 space-y-4 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 text-xs">
      <h3 class="text-sm font-bold text-zinc-900 dark:text-white">新建母版积木 (Component Template)</h3>
      <div class="space-y-3">
        <label class="block">
          <span class="mb-1 block text-zinc-500">积木名称</span>
          <input bind:value={name} placeholder="通用节点出站模版" class="w-full rounded border border-zinc-300 bg-white px-2.5 py-1.5 dark:border-zinc-700 dark:bg-zinc-950" />
        </label>

        <div>
          <span class="mb-1 block text-zinc-500">挂载点快捷预设:</span>
          <div class="flex flex-wrap gap-1.5">
            <!-- 加上 (cap.path) 解决 require-each-key -->
            {#each PRESET_CAPSULES as cap (cap.path)}
              <button
                type="button"
                onclick={() => selectCapsule(cap)}
                class="rounded-md border px-2 py-0.5 font-mono text-[10px] transition cursor-pointer {mountPath === cap.path ? 'border-blue-500 bg-blue-50 text-blue-600 dark:bg-blue-950/60 dark:text-blue-400 font-bold' : 'border-zinc-200 bg-zinc-50 text-zinc-600 hover:bg-zinc-100 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-400'}"
              >
                {cap.label}
              </button>
            {/each}
          </div>
        </div>

        <div class="grid grid-cols-3 gap-2">
          <label class="block">
            <span class="mb-1 block text-zinc-500">挂载点 (绝对路径)</span>
            <input bind:value={mountPath} placeholder="route.rules" class="w-full rounded border border-zinc-300 bg-white px-2.5 py-1.5 font-mono dark:border-zinc-700 dark:bg-zinc-950" />
          </label>
          <label class="block">
            <span class="mb-1 block text-zinc-500">合并策略</span>
            <select bind:value={strategy} class="w-full rounded border border-zinc-300 bg-white px-2 py-1.5 dark:border-zinc-700 dark:bg-zinc-950">
              <option value="append">append (推入数组)</option>
              <option value="merge">merge (键值合入)</option>
            </select>
          </label>
          <label class="block">
            <span class="mb-1 block text-zinc-500">适用标签 (逗号分隔)</span>
            <input bind:value={scopeTag} placeholder="universal" class="w-full rounded border border-zinc-300 bg-white px-2.5 py-1.5 font-mono dark:border-zinc-700 dark:bg-zinc-950" />
          </label>
        </div>
      </div>

      <div class="flex justify-end gap-2 pt-2">
        <button onclick={onClose} class="rounded px-3 py-1.5 text-zinc-400 hover:text-zinc-800 dark:hover:text-white cursor-pointer">取消</button>
        <button
          onclick={() => onCreate({ name, category, scope_tag: scopeTag, mount_path: mountPath, strategy, payload_tmpl: payloadTmpl })}
          class="rounded bg-blue-600 px-4 py-1.5 font-medium text-white hover:bg-blue-500 cursor-pointer"
        >
          保存并进入工坊
        </button>
      </div>
    </div>
  </div>
{/if}