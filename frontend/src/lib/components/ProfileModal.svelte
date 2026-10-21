<!-- frontend/src/lib/components/ProfileModal.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Profile } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    open?: boolean;
    editProfile?: Profile | null;
    onClose?: () => void;
    onSave?: (data: {
      id: string;
      name: string;
      token: string;
      tags: string[];
      description: string;
      sing_box_version: string;
      enable_subscribe: boolean;
      enable_ping: boolean;
      enable_push: boolean;
    }) => void;
  }

  let {
    open = false,
    editProfile = null,
    onClose = () => {},
    onSave = () => {}
  }: Props = $props();

  let id = $state('');
  let name = $state('');
  let token = $state('');
  let tagsStr = $state('');
  let description = $state('');
  let singBoxVersion = $state('1.10.0');
  let enableSubscribe = $state(true);
  let enablePing = $state(true);
  let enablePush = $state(true);

  $effect(() => {
    if (open) {
      if (editProfile) {
        id = editProfile.id;
        name = editProfile.name;
        token = editProfile.token;
        tagsStr = (editProfile.tags || []).join(', ');
        description = editProfile.description;
        singBoxVersion = editProfile.sing_box_version || '1.10.0';
        enableSubscribe = editProfile.capabilities?.enable_subscribe ?? true;
        enablePing = editProfile.capabilities?.enable_ping ?? true;
        enablePush = editProfile.capabilities?.enable_push ?? true;
      } else {
        id = 'prof-' + Date.now().toString(36);
        // 核心优化：新建时给一个默认名称，防止空值拦截
        name = '新建方案配方';
        token = crypto.randomUUID().replace(/-/g, '').slice(0, 16);
        tagsStr = 'default';
        description = '';
        singBoxVersion = '1.10.0';
        enableSubscribe = true;
        enablePing = true;
        enablePush = true;
      }
    }
  });

  function handleSubmit() {
    if (!name.trim()) {
      ws.showToast('请输入方案名称', 'error');
      return;
    }
    onSave({
      id: id || ('prof-' + Date.now().toString(36)),
      name: name.trim(),
      token: token.trim(),
      tags: tagsStr.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean),
      description: description.trim(),
      sing_box_version: singBoxVersion.trim() || '1.10.0',
      enable_subscribe: enableSubscribe,
      enable_ping: enablePing,
      enable_push: enablePush,
    });
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4 animate-in fade-in duration-150">
    <div class="w-full max-w-md rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800">
        <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
          <Icon name="layers" color="amber" size={17} />
          <span>{editProfile ? '编辑方案元数据' : '新建方案配方'}</span>
        </h3>
        <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={15} />
        </button>
      </div>

      <div class="space-y-3.5 text-xs">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">
              方案名称 <span class="text-rose-500 font-bold">*</span>
            </span>
            <input
              bind:value={name}
              placeholder="如: 全网分流出海方案"
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">方案标签 (逗号分隔)</span>
            <input
              bind:value={tagsStr}
              placeholder="如: prod, home, vps"
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">订阅安全 Token</span>
            <input
              bind:value={token}
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>

          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">目标 Sing-box 版本</span>
            <input
              bind:value={singBoxVersion}
              placeholder="1.10.0"
              class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </label>
        </div>

        <div class="border-t border-zinc-100 pt-3 dark:border-zinc-800 space-y-2">
          <span class="block font-medium text-zinc-700 dark:text-zinc-300">方案能力开关</span>
          <div class="flex items-center gap-4">
            <label class="flex items-center gap-1.5 cursor-pointer">
              <input type="checkbox" bind:checked={enableSubscribe} class="rounded" />
              <span>允许订阅</span>
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer">
              <input type="checkbox" bind:checked={enablePing} class="rounded" />
              <span>允许测活</span>
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer">
              <input type="checkbox" bind:checked={enablePush} class="rounded" />
              <span>允许推送</span>
            </label>
          </div>
        </div>
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800">
        <button onclick={onClose} type="button" class="rounded-lg border border-zinc-300 px-3.5 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 cursor-pointer">
          取消
        </button>
        <button onclick={handleSubmit} type="button" class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer">
          保存方案
        </button>
      </div>
    </div>
  </div>
{/if}