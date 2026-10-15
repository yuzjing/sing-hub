<!-- frontend/src/lib/components/ProfileModal.svelte -->
<script lang="ts">
  import type { Profile } from '$lib/types';

  let {
    open = false,
    editProfile = null,
    onClose,
    onSave,
  }: {
    open: boolean;
    editProfile?: Profile | null;
    onClose: () => void;
    onSave: (p: {
      id: string;
      name: string;
      token: string;
      description: string;
      sing_box_version: string;
      enable_subscribe: boolean;
      enable_ping: boolean;
      enable_push: boolean;
    }) => void;
  } = $props();

  let id = $state('');
  let name = $state('');
  let token = $state('');
  let description = $state('');
  let singBoxVersion = $state('1.11.0');
  let enableSub = $state(true);
  let enablePing = $state(true);
  let enablePush = $state(false);

  $effect(() => {
    if (editProfile) {
      id = editProfile.id;
      name = editProfile.name;
      token = editProfile.token;
      description = editProfile.description || '';
      singBoxVersion = editProfile.sing_box_version || '1.11.0';
      enableSub = editProfile.capabilities?.enable_subscribe ?? true;
      enablePing = editProfile.capabilities?.enable_ping ?? true;
      enablePush = editProfile.capabilities?.enable_push ?? false;
    } else {
      id = '';
      name = '';
      token = crypto.randomUUID().replace(/-/g, '').slice(0, 16);
      description = '';
      singBoxVersion = '1.11.0';
      enableSub = true;
      enablePing = true;
      enablePush = false;
    }
  });

  const VERSION_CAPSULES = ['1.12.x', '1.11.x', '1.10.x'];
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
    <div class="w-96 rounded-xl border border-zinc-200 bg-white p-5 space-y-4 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 text-xs">
      <h3 class="text-sm font-bold text-zinc-900 dark:text-white">
        {editProfile ? '编辑方案元数据' : '新建方案配方 (Profile)'}
      </h3>
      <div class="space-y-3">
        <label class="block">
          <span class="mb-1 block text-zinc-500">方案唯一标识 (ID)</span>
          <input
            disabled={!!editProfile}
            bind:value={id}
            placeholder="prod-streaming"
            class="w-full rounded border border-zinc-300 bg-white px-2.5 py-1.5 font-mono dark:border-zinc-700 dark:bg-zinc-950 disabled:opacity-50"
          />
        </label>

        <label class="block">
          <span class="mb-1 block text-zinc-500">方案名称</span>
          <input bind:value={name} placeholder="通用串流与组网方案" class="w-full rounded border border-zinc-300 bg-white px-2.5 py-1.5 dark:border-zinc-700 dark:bg-zinc-950" />
        </label>

        <!-- 目标 sing-box 内核版本选用 -->
        <label class="block">
          <div class="flex items-center justify-between mb-1">
            <span class="text-zinc-500">适配内核版本 (sing-box version)</span>
            <div class="flex gap-1">
              {#each VERSION_CAPSULES as ver}
                <button
                  type="button"
                  onclick={() => (singBoxVersion = ver.replace('.x', '.0'))}
                  class="rounded bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.2 text-[10px] text-zinc-600 dark:text-zinc-400 hover:text-blue-500 font-mono"
                >
                  {ver}
                </button>
              {/each}
            </div>
          </div>
          <input bind:value={singBoxVersion} placeholder="1.11.0" class="w-full rounded border border-zinc-300 bg-white px-2.5 py-1.5 font-mono dark:border-zinc-700 dark:bg-zinc-950" />
        </label>

        <div class="space-y-1.5 pt-2 border-t border-zinc-100 dark:border-zinc-800">
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={enableSub} />
            <span>开启客户端订阅端点 (/sub/{token || 'token'})</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={enablePing} />
            <span>开启节点握手延迟探测</span>
          </label>
        </div>
      </div>

      <div class="flex justify-end gap-2 pt-2">
        <button onclick={onClose} class="rounded px-3 py-1.5 text-zinc-400 hover:text-white cursor-pointer">取消</button>
        <button
          onclick={() => onSave({
            id,
            name,
            token,
            description,
            sing_box_version: singBoxVersion,
            enable_subscribe: enableSub,
            enable_ping: enablePing,
            enable_push: enablePush,
          })}
          class="rounded bg-blue-600 px-4 py-1.5 font-medium text-white hover:bg-blue-500 cursor-pointer"
        >
          {editProfile ? '保存修改' : '创建方案'}
        </button>
      </div>
    </div>
  </div>
{/if}