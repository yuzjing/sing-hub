<!-- frontend/src/lib/components/DeviceModal.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Device, DeviceProp } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  interface Props {
    open?: boolean;
    editDevice?: Device | null;
    onClose?: (e?: MouseEvent) => void;
    onSave?: (d: Device) => void;
  }

  let { open = false, editDevice = null, onClose = () => {}, onSave = () => {} }: Props = $props();

  let id = $state('');
  let name = $state('');
  let targetProfileId = $state('');
  let sshHost = $state('');
  let sshPort = $state(22);
  let sshUser = $state('root');
  let authType = $state('key_path');
  let authSecret = $state('');
  let keyPath = $state('~/.ssh/id_rsa');
  let remotePath = $state('~/.config/sing-box/config.json');
  // 新增部署策略
  let deployStrategy = $state('systemd');
  let serviceName = $state('sing-box');
  
  let tagsStr = $state('');
  let enablePing = $state(true);
  let enablePush = $state(true);
  let description = $state('');
  let dynamicProps = $state<DeviceProp[]>([]);

  $effect(() => {
    if (open) {
      if (editDevice) {
        id = editDevice.id;
        name = editDevice.name;
        targetProfileId = editDevice.target_profile_id || '';
        sshHost = editDevice.ssh_host || '';
        sshPort = editDevice.ssh_port || 22;
        sshUser = editDevice.ssh_user || 'root';
        authType = editDevice.auth_type || 'key_path';
        authSecret = editDevice.auth_secret || '';
        keyPath = editDevice.key_path || '~/.ssh/id_rsa';
        remotePath = editDevice.remote_path || '~/.config/sing-box/config.json';
        deployStrategy = editDevice.deploy_strategy || 'systemd';
        serviceName = editDevice.service_name || 'sing-box';
        tagsStr = (editDevice.tags || []).join(', ');
        enablePing = editDevice.enable_ping ?? true;
        enablePush = editDevice.enable_push ?? true;
        description = editDevice.description || '';
        dynamicProps = (editDevice.props && editDevice.props.length > 0)
          ? JSON.parse(JSON.stringify(editDevice.props))
          : (editDevice.endpoints || []).map((ep: string, idx: number) => ({
              key: idx === 0 ? 'endpoint' : 'endpoint_' + (idx + 1),
              label: idx === 0 ? '主端点' : '备用端点 ' + (idx + 1),
              value: ep
            }));
      } else {
        id = '';
        name = '';
        targetProfileId = ws.profiles[0]?.id || '';
        sshHost = '';
        sshPort = 22;
        sshUser = 'root';
        authType = 'key_path';
        authSecret = '';
        keyPath = '~/.ssh/id_rsa';
        remotePath = '~/.config/sing-box/config.json';
        deployStrategy = 'systemd';
        serviceName = 'sing-box';
        tagsStr = 'vps';
        enablePing = true;
        enablePush = true;
        description = '';
        dynamicProps = [{ key: 'endpoint', label: '主通信端点', value: '' }];
      }
    }
  });

  function addPropRow() {
    dynamicProps = [...dynamicProps, { key: 'prop_' + (dynamicProps.length + 1), label: '', value: '' }];
  }

  function removePropRow(idx: number) {
    dynamicProps = dynamicProps.filter((_: DeviceProp, i: number) => i !== idx);
  }

  function handleSave() {
    if (!name.trim()) {
      ws.showToast('请输入节点名称', 'error');
      return;
    }
    let finalId = id.trim() || 'node-' + Date.now().toString(36);

    const endpoints = dynamicProps.map((p: DeviceProp) => p.value.trim()).filter((v: string) => v !== '');

    onSave({
      id: finalId,
      name: name.trim(),
      endpoints,
      props: dynamicProps.filter((p: DeviceProp) => p.key.trim() !== ''),
      ssh_host: sshHost.trim(),
      ssh_port: sshPort || 22,
      ssh_user: sshUser.trim() || 'root',
      auth_type: authType,
      auth_secret: authSecret,
      key_path: keyPath.trim(),
      remote_path: remotePath.trim() || '~/.config/sing-box/config.json',
      deploy_strategy: deployStrategy,
      service_name: serviceName.trim() || 'sing-box',
      target_profile_id: targetProfileId,
      tags: tagsStr.split(/[,，\s]+/).map((s: string) => s.trim()).filter(Boolean),
      enable_ping: enablePing,
      enable_push: enablePush,
      description: description.trim(),
    });
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4 animate-in fade-in duration-150">
    <div class="w-full max-w-2xl rounded-2xl border border-zinc-200 bg-white p-6 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 space-y-4 max-h-[90vh] flex flex-col">
      <div class="flex items-center justify-between border-b border-zinc-100 pb-3 dark:border-zinc-800 shrink-0">
        <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
          <Icon name="server" color="blue" size={17} />
          <span>{editDevice ? '编辑网络节点与实例' : '接入新网络节点'}</span>
        </h3>
        <button onclick={onClose} type="button" class="rounded p-1 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer">
          <Icon name="close" size={15} />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto space-y-4 text-xs pr-1">
        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">节点唯一 ID</span>
            <input bind:value={id} disabled={!!editDevice} placeholder="留空自动生成" class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 disabled:opacity-50 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          </label>
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">节点名称</span>
            <input bind:value={name} placeholder="例如: 香港 CN2 高速节点" class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          </label>
        </div>

        <div class="rounded-xl border border-zinc-200 bg-zinc-50/60 p-3.5 dark:border-zinc-800 dark:bg-zinc-950/60 space-y-2.5">
          <div class="flex items-center justify-between">
            <div>
              <span class="font-bold text-zinc-700 dark:text-zinc-300 block">网络端点、多域名与自定义属性表:</span>
            </div>
            <button onclick={addPropRow} type="button" class="rounded-md border border-zinc-300 bg-white dark:border-zinc-700 dark:bg-zinc-900 px-2.5 py-1 text-[11px] font-semibold text-blue-600 hover:bg-zinc-50 cursor-pointer shadow-2xs flex items-center gap-1">
              <Icon name="plus" size={11} /> <span>添加行</span>
            </button>
          </div>
          <div class="space-y-1.5">
            {#each dynamicProps as prop, idx (idx)}
              <div class="flex items-center gap-2">
                <input bind:value={prop.key} placeholder="标识(如 domain)" class="w-36 rounded border border-zinc-300 bg-white px-2 py-1 text-[11px] font-mono dark:border-zinc-700 dark:bg-zinc-950" />
                <input bind:value={prop.label} placeholder="备注(如 Reality)" class="w-32 rounded border border-zinc-300 bg-white px-2 py-1 text-[11px] dark:border-zinc-700 dark:bg-zinc-950" />
                <input bind:value={prop.value} placeholder="地址/域名" class="flex-1 rounded border border-zinc-300 bg-white px-2 py-1 text-[11px] font-mono dark:border-zinc-700 dark:bg-zinc-950" />
                <button onclick={() => removePropRow(idx)} type="button" class="p-1 text-zinc-400 hover:text-rose-500 cursor-pointer"><Icon name="close" size={13} /></button>
              </div>
            {/each}
          </div>
        </div>

        <!-- 部署策略与凭据面板 -->
        <div class="rounded-xl border border-blue-200/60 bg-blue-50/30 p-3.5 dark:border-blue-900/40 dark:bg-blue-950/20 space-y-3">
          <div class="flex items-center justify-between border-b border-blue-100 pb-2 dark:border-blue-900/50">
            <span class="font-bold text-blue-800 dark:text-blue-300 text-sm">SSH 自动化运维与推送策略</span>
            <div class="flex items-center gap-4">
              <label class="flex items-center gap-1.5 cursor-pointer text-blue-800 dark:text-blue-300 font-medium">
                <input type="checkbox" bind:checked={enablePing} class="rounded" /> 开启测活
              </label>
              <label class="flex items-center gap-1.5 cursor-pointer text-blue-800 dark:text-blue-300 font-medium">
                <input type="checkbox" bind:checked={enablePush} class="rounded" /> 开启推送
              </label>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <label class="block">
              <span class="block text-[11px] text-zinc-600 dark:text-zinc-400 mb-1">节点运行策略 (Strategy)</span>
              <select bind:value={deployStrategy} class="w-full rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500">
                <option value="systemd">原生 Systemd 服务</option>
                <option value="podman">Podman 容器</option>
                <option value="docker">Docker 容器</option>
                <option value="standalone">仅二进制测试 (不重启)</option>
              </select>
            </label>
            <label class="block">
              <span class="block text-[11px] text-zinc-600 dark:text-zinc-400 mb-1">目标服务名 / 容器名</span>
              <input bind:value={serviceName} placeholder="如: sing-box" class="w-full rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500" />
            </label>
          </div>

          <div class="grid grid-cols-3 gap-2.5">
            <label class="block">
              <span class="block text-[11px] text-zinc-500 mb-1">SSH 主机 / IP</span>
              <input bind:value={sshHost} placeholder="留空使用主端点" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
            <label class="block">
              <span class="block text-[11px] text-zinc-500 mb-1">SSH 用户</span>
              <input bind:value={sshUser} placeholder="root" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
            <label class="block">
              <span class="block text-[11px] text-zinc-500 mb-1">SSH 端口</span>
              <input type="number" bind:value={sshPort} placeholder="22" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
          </div>

          <div class="grid grid-cols-2 gap-2.5">
            <label class="block">
              <span class="block text-[11px] text-zinc-500 mb-1">SSH 认证私钥路径</span>
              <input bind:value={keyPath} placeholder="~/.ssh/id_rsa" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
            <label class="block">
              <span class="block text-[11px] text-zinc-500 mb-1">远端配置文件下发路径</span>
              <input bind:value={remotePath} placeholder="~/.config/sing-box/config.json" class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950" />
            </label>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">指派生效方案</span>
            <select bind:value={targetProfileId} class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs dark:border-zinc-700 dark:bg-zinc-950">
              <option value="">-- 未指派方案 --</option>
              {#each ws.profiles as p (p.id)}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          </label>
          <label class="block">
            <span class="block font-medium text-zinc-700 dark:text-zinc-300 mb-1">节点标签</span>
            <input bind:value={tagsStr} placeholder="如: hk, prod" class="w-full rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950" />
          </label>
        </div>
      </div>

      <div class="flex justify-end gap-2 border-t border-zinc-100 pt-3 dark:border-zinc-800 shrink-0">
        <button onclick={onClose} type="button" class="rounded-lg border border-zinc-300 px-3.5 py-1.5 text-xs text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 cursor-pointer">取消</button>
        <button onclick={handleSave} type="button" class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-2xs">保存节点</button>
      </div>
    </div>
  </div>
{/if}