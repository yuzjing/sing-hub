<!-- frontend/src/lib/components/ConfirmModal.svelte -->
<script lang="ts">
  let {
    open = false,
    title = '确认操作',
    content = '',
    actionType = 'danger',
    onConfirm,
    onCancel,
  }: {
    open: boolean;
    title: string;
    content: string;
    actionType?: 'danger' | 'primary';
    onConfirm: () => void;
    onCancel: () => void;
  } = $props();
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
    <div class="w-96 rounded-xl border border-zinc-200 bg-white p-5 space-y-4 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 text-xs">
      <h3 class="text-sm font-bold text-zinc-900 dark:text-white">{title}</h3>
      <p class="text-zinc-500 dark:text-zinc-400 leading-relaxed">{content}</p>
      <div class="flex justify-end gap-2 pt-2">
        <button
          onclick={onCancel}
          class="rounded-lg px-3 py-1.5 text-zinc-400 hover:text-zinc-800 dark:hover:text-white cursor-pointer"
        >
          取消
        </button>
        <button
          onclick={onConfirm}
          class="rounded-lg px-4 py-1.5 font-semibold text-white cursor-pointer transition {actionType === 'danger' ? 'bg-rose-600 hover:bg-rose-500' : 'bg-blue-600 hover:bg-blue-500'}"
        >
          确认
        </button>
      </div>
    </div>
  </div>
{/if}