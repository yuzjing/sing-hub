<!-- frontend/src/lib/components/ConfirmModal.svelte -->
<script lang="ts">
  import Icon from '$lib/components/Icon.svelte';

  let {
    open = false,
    title = '确认此操作？',
    content = '此操作将不可逆，请确认是否继续。',
    actionType = 'danger',
    onConfirm = () => {},
    onCancel = () => {},
  } = $props<{
    open?: boolean;
    title?: string;
    content?: string;
    actionType?: 'danger' | 'primary';
    onConfirm?: () => void;
    onCancel?: () => void;
  }>();
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
    <div class="w-full max-w-sm rounded-2xl border border-zinc-200 bg-white p-5 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-4">
      <div class="flex items-start gap-3.5">
        <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl {actionType === 'danger' ? 'bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400 border border-rose-200 dark:border-rose-900/50' : 'bg-blue-50 text-blue-600 dark:bg-blue-950/60 dark:text-blue-400 border border-blue-200 dark:border-blue-900/50'}">
          <Icon name={actionType === 'danger' ? 'trash' : 'sliders'} size={17} />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100">{title}</h3>
          <p class="text-xs text-zinc-500 dark:text-zinc-400 leading-relaxed">{content}</p>
        </div>
      </div>

      <div class="flex items-center justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800/80">
        <button
          onclick={onCancel}
          class="rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-800 px-3.5 py-1.5 text-xs font-medium text-zinc-700 dark:text-zinc-200 hover:bg-zinc-50 dark:hover:bg-zinc-700/60 transition cursor-pointer shadow-2xs"
        >
          取消
        </button>
        <button
          onclick={onConfirm}
          class="rounded-lg px-4 py-1.5 text-xs font-semibold text-white shadow-xs transition cursor-pointer {actionType === 'danger' ? 'bg-rose-600 hover:bg-rose-500 shadow-rose-600/20' : 'bg-blue-600 hover:bg-blue-500 shadow-blue-600/20'}"
        >
          确认
        </button>
      </div>
    </div>
  </div>
{/if}