<!-- frontend/src/lib/components/DeployModal.svelte -->
<script lang="ts">
  import Icon from '$lib/components/Icon.svelte';
  
  interface Props {
    open?: boolean;
    deviceName?: string;
    logs?: string;
    status?: 'running' | 'success' | 'error';
    onClose?: () => void;
  }

  let {
    open = false,
    deviceName = '',
    logs = '',
    status = 'running',
    onClose = () => {}
  }: Props = $props();

  // 修复非响应式绑定警告
  let logContainer = $state<HTMLDivElement | null>(null);

  $effect(() => {
    if (logs && logContainer) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  });
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
    <div class="w-full max-w-2xl rounded-2xl border border-zinc-700 bg-zinc-950 p-6 shadow-2xl space-y-4 flex flex-col">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <h3 class="text-sm font-bold text-zinc-100 flex items-center gap-2">
          <Icon name="terminal" color="blue" size={17} />
          <span>部署至节点: {deviceName}</span>
        </h3>
        {#if status !== 'running'}
          <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-800 cursor-pointer">
            <Icon name="close" size={15} />
          </button>
        {/if}
      </div>

      <div class="flex items-center gap-2 text-xs font-mono">
        {#if status === 'running'}
          <span class="flex items-center gap-1.5 text-blue-400"><Icon name="refresh" size={14} class="animate-spin" /> 执行中...</span>
        {:else if status === 'success'}
          <span class="text-emerald-400">✓ 部署成功</span>
        {:else}
          <span class="text-rose-400">✗ 部署失败</span>
        {/if}
      </div>

      <div bind:this={logContainer} class="h-64 w-full rounded-xl bg-black p-4 text-[11px] font-mono text-zinc-300 overflow-y-auto border border-zinc-800 shadow-inner whitespace-pre-wrap">
        {logs || 'Waiting for SSH connection...'}
      </div>

      {#if status !== 'running'}
        <div class="flex justify-end pt-2">
          <button onclick={onClose} type="button" class="rounded-lg bg-zinc-800 px-5 py-2 text-xs font-bold text-white hover:bg-zinc-700 cursor-pointer">
            关闭
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}