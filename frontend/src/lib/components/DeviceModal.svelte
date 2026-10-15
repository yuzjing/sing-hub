<!-- frontend/src/lib/components/DeviceModal.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Device } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  let {
    open = false,
    editDevice = null,
    onClose,
    onSave,
  }: {
    open: boolean;
    editDevice?: Device | null;
    onClose: () => void;
    onSave: (dev: Device) => void;
  } = $props();

  let id = $state('');
  let name = $state('');
  let endpointsStr = $state('');
  let sshHost = $state('');
  let profileId = $state('');
  let authType = $state<'key_path' | 'key_content' | 'password'>('key_path');
  let keyPath = $state('/root/.ssh/id_ed25519');
  let secret = $state('');
  let sshUser = $state('root');
  let sshPort = $state(22);
  let remotePath = $state('~/.config/sing-box/config.json');
  let tagsStr = $state('');
  let enablePing = $state(true);
  let enablePush = $state(true);

  let isIdValid = $derived.by(() => {
    if (editDevice) return true;
    return /^[a-z0-9_-]+$/.test(id.trim());
  });

  $effect(() => {
    if (editDevice) {
      id = editDevice.id;
      name = editDevice.name;
      endpointsStr = (editDevice.endpoints || []).join('\n');
      sshHost = editDevice.ssh_host || '';
      profileId = editDevice.target_profile_id || '';
      authType = editDevice.auth_type || 'key_path';
      keyPath = editDevice.key_path || '/root/.ssh/id_ed25519';
      secret = editDevice.auth_secret || '';
      sshUser = editDevice.ssh_user || 'root';
      sshPort = editDevice.ssh_port || 22;
      remotePath = editDevice.remote_path || '~/.config/sing-box/config.json';
      tagsStr = (editDevice.tags || []).join(', ');
      enablePing = editDevice.enable_ping;
      enablePush = editDevice.enable_push;
    } else {
      id = '';
      name = '';
      endpointsStr = '';
      sshHost = '';
      profileId = '';
      authType = 'key_path';
      keyPath = '/root/.ssh/id_ed25519';
      secret = '';
      sshUser = 'root';
      sshPort = 22;
      remotePath = '~/.config/sing-box/config.json';
      tagsStr = 'vps, prod';
      enablePing = true;
      enablePush = true;
    }
  });

  let candidateAddresses = $derived.by(() => {
    return endpointsStr.split(/[\n,]/).map(s => s.trim()).filter(Boolean);
  });
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
    <div class="w-120 rounded-2xl border border-zinc-200 bg-white p-6 space-y-4 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 text-xs max-h-[90vh] overflow-y-auto">
      <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800">
        <div class="flex items-center gap-2 font-semibold text-sm text-zinc-900 dark:text-white">
          <Icon name="server" size={18} color="blue" />
          <span>{editDevice ? '编辑网络节点实例' : '接入新网络节点'}</span>
        </div>
        <button onclick={onClose} class="text-zinc-400 hover:text-zinc-600 dark:hover:text-white cursor-pointer">
          <Icon name="close" size={16} />
        </button>
      </div>

      <div class="space-y-3.5">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="mb-1 block font-medium text-zinc-500">节点标识 (ID)</span>
            <input disabled={!!editDevice} bind:value={id} placeholder="vps-sg-01" class="w-full rounded-lg border bg-white px-3 py-1.5 font-mono text-xs dark:bg-zinc-950 transition {editDevice ? 'border-zinc-200 dark:border-zinc-800 opacity-60' : isIdValid ? 'border-zinc-300 dark:border-zinc-700' : 'border-rose-500 ring-1 ring-rose-500/20'}" />
            {#if !editDevice && id && !isIdValid}
              <span class="text-[10px] text-rose-500 mt-0.5 block">只允许小写英文字母、数字、下划线及连字符</span>
            {/if}
          </label>
          <label class="block">
            <span class="mb-1 block font-medium text-zinc-500">显示名称 (Name)</span>
            <input bind:value={name} placeholder="生产网关 01" class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950" />
          </label>
        </div>

        <label class="block">
          <span class="text-zinc-500 font-medium block mb-1">通信地址列表 (支持双栈 IPv4/IPv6/域名，换行隔开)</span>
          <textarea bind:value={endpointsStr} rows="2" placeholder="sg.domain.com&#10;2400:cb00:2048:1::c629:d7a2" class="w-full rounded-lg border border-zinc-300 bg-white p-2 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950"></textarea>
        </label>

        <label class="block">
          <span class="text-zinc-500 font-medium block mb-1">专属 SSH 运维目标 (为空则默认首选地址)</span>
          <input bind:value={sshHost} placeholder="独立内网 IP 或特定域名" class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950" />
          {#if candidateAddresses.length > 0}
            <div class="flex items-center gap-1.5 mt-1.5 text-[10px] text-zinc-400">
              <span>快捷选用:</span>
              <!-- 加上 (addr) 解决 require-each-key -->
              {#each candidateAddresses as addr (addr)}
                <button type="button" onclick={() => (sshHost = addr)} class="rounded bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 hover:text-blue-500 font-mono transition">{addr}</button>
              {/each}
            </div>
          {/if}
        </label>

        <label class="block">
          <span class="mb-1 block font-medium text-zinc-500">指派生效的方案配方 (Target Profile)</span>
          <select bind:value={profileId} class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 dark:border-zinc-700 dark:bg-zinc-950">
            <option value="">-- 暂不指派 --</option>
            {#each ws.profiles as p (p.id)}
              <option value={p.id}>{p.name}</option>
            {/each}
          </select>
        </label>

        <div class="border-t border-zinc-100 pt-3 dark:border-zinc-800 space-y-2.5">
          <span class="font-semibold block text-zinc-700 dark:text-zinc-300">SSH 自动化凭据</span>
          <div class="grid grid-cols-3 gap-2">
            <label class="block">
              <span class="text-zinc-400 block mb-0.5 text-[11px]">认证模式</span>
              <select bind:value={authType} class="w-full rounded border border-zinc-300 bg-white px-2 py-1 dark:border-zinc-700 dark:bg-zinc-950">
                <option value="key_path">宿主私钥文件</option>
                <option value="key_content">私钥文本</option>
                <option value="password">密码</option>
              </select>
            </label>
            <label class="block">
              <span class="text-zinc-400 block mb-0.5 text-[11px]">用户名</span>
              <input bind:value={sshUser} placeholder="root" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
            <label class="block">
              <span class="text-zinc-400 block mb-0.5 text-[11px]">端口</span>
              <input type="number" bind:value={sshPort} class="w-full rounded border border-zinc-300 bg-white px-2 py-1 font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
          </div>

          {#if authType === 'key_path'}
            <label class="block">
              <span class="text-zinc-400 block mb-0.5 text-[11px]">宿主机私钥物理路径</span>
              <input bind:value={keyPath} placeholder="/root/.ssh/id_ed25519" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
          {:else if authType === 'key_content'}
            <label class="block">
              <span class="text-zinc-400 block mb-0.5 text-[11px]">私钥内容</span>
              <textarea bind:value={secret} rows="2" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" class="w-full rounded border border-zinc-300 bg-white p-2 font-mono text-[10px] dark:border-zinc-700 dark:bg-zinc-950"></textarea>
            </label>
          {:else}
            <label class="block">
              <span class="text-zinc-400 block mb-0.5 text-[11px]">SSH 密码</span>
              <input type="password" bind:value={secret} placeholder="输入登录密码" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
          {/if}
        </div>

        <label class="block">
          <span class="mb-1 block font-medium text-zinc-500">分类标签 (逗号分隔)</span>
          <input bind:value={tagsStr} placeholder="vps, sg, prod" class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 font-mono text-xs dark:border-zinc-700 dark:bg-zinc-950" />
        </label>

        <div class="grid grid-cols-2 gap-2 pt-2 border-t border-zinc-100 dark:border-zinc-800">
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={enablePing} class="rounded" />
            <span>开启一键并发探活</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={enablePush} class="rounded" />
            <span>允许执行远程推送</span>
          </label>
        </div>
      </div>

      <div class="flex justify-end gap-2 pt-3 border-t border-zinc-100 dark:border-zinc-800">
        <button onclick={onClose} class="rounded-lg px-3 py-1.5 text-zinc-500 hover:text-zinc-800 dark:hover:text-white cursor-pointer">取消</button>
        <button
          disabled={!id || !isIdValid}
          onclick={() => onSave({
            id: id.trim(),
            name: name.trim() || id.trim(),
            endpoints: candidateAddresses,
            ssh_host: sshHost.trim(),
            target_profile_id: profileId,
            auth_type: authType,
            auth_secret: secret,
            key_path: keyPath,
            ssh_user: sshUser.trim() || 'root',
            ssh_port: Number(sshPort) || 22,
            remote_path: remotePath.trim() || '~/.config/sing-box/config.json',
            tags: tagsStr.split(/[\n,]/).map(s => s.trim()).filter(Boolean),
            enable_ping: enablePing,
            enable_push: enablePush,
            description: ''
          })}
          class="rounded-lg bg-blue-600 px-4 py-1.5 font-semibold text-white hover:bg-blue-500 disabled:opacity-40 cursor-pointer shadow-sm transition"
        >
          {editDevice ? '保存修改' : '接入节点'}
        </button>
      </div>
    </div>
  </div>
{/if}