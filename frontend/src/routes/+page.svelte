<!-- frontend/src/routes/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Component, Module, Profile, Device } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  // 引入拆分后的 4 个独立工作区
  import StudioWorkspace from '$lib/components/workspaces/StudioWorkspace.svelte';
  import ModulesWorkspace from '$lib/components/workspaces/ModulesWorkspace.svelte';
  import FoundryWorkspace from '$lib/components/workspaces/FoundryWorkspace.svelte';
  import FleetWorkspace from '$lib/components/workspaces/FleetWorkspace.svelte';

  // 模态框与抽屉（使用拆分后的 Meta 与 Params 弹窗，不再引用旧 ModuleModal）
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import MountDrawer from '$lib/components/MountDrawer.svelte';
  import SettingsDrawer from '$lib/components/SettingsDrawer.svelte';
  import ComponentModal from '$lib/components/ComponentModal.svelte';
  import ModuleMetaModal from '$lib/components/ModuleMetaModal.svelte';
  import ModuleParamsModal from '$lib/components/ModuleParamsModal.svelte';
  import DeviceModal from '$lib/components/DeviceModal.svelte';
  import ProfileModal from '$lib/components/ProfileModal.svelte';

  let showMountDrawer = $state(false);

  // 1. 方案配方 (PROFILES)
  let isProfileManageMode = $state(false);
  let selectedProfileIds = $state<string[]>([]);
  let editingProfile = $state<Profile | null>(null);
  let showProfileModal = $state(false);

  // 2. 成品积木 (MODULES)
  let activeModuleId = $state<string>('');
  let isModuleManageMode = $state(false);
  let selectedModuleIds = $state<string[]>([]);
  let editingModule = $state<Module | null>(null);
  let showModuleMetaModal = $state(false);
  let tuningModule = $state<Module | null>(null);
  let showModuleParamsModal = $state(false);

  // 3. 语法母版 (FOUNDRY)
  let studioActiveCompId = $state<string>('');
  let isCompManageMode = $state(false);
  let selectedCompIds = $state<string[]>([]);
  let editingComponent = $state<Component | null>(null);
  let showNewCompModal = $state(false);

  // 4. 节点运维 (FLEET)
  let editingDevice = $state<Device | null>(null);
  let isDeviceManageMode = $state(false);
  let selectedDeviceIds = $state<string[]>([]);
  let showDeviceModal = $state(false);

  // 确认对话框：加入 finally 自动关闭
  let confirmState = $state({
    open: false,
    title: '',
    content: '',
    actionType: 'danger' as 'danger' | 'primary',
    onConfirm: () => {},
  });

  function triggerConfirm(title: string, content: string, onConfirm: () => void | Promise<void>, actionType: 'danger' | 'primary' = 'danger') {
    confirmState = {
      open: true,
      title,
      content,
      actionType,
      onConfirm: async () => {
        try {
          await onConfirm();
        } finally {
          confirmState.open = false;
        }
      },
    };
  }

  onMount(() => {
    ws.init();
  });

  async function handleSaveProfile(data: { id: string; name: string; token: string; description: string; sing_box_version: string; enable_subscribe: boolean; enable_ping: boolean; enable_push: boolean }) {
    const p: Profile = {
      ...data,
      capabilities: {
        enable_subscribe: data.enable_subscribe,
        enable_ping: data.enable_ping,
        enable_push: data.enable_push,
      },
      modules: editingProfile ? editingProfile.modules : [],
    };
    await api.saveProfile(p);
    await ws.loadProfiles();
    ws.selectProfile(p.id);
    showProfileModal = false;
    editingProfile = null;
    ws.showToast('方案保存成功 ✓', 'success');
  }

  async function handleSaveComponent(c: Component) {
    await api.saveComponent(c);
    await ws.loadComponents();
    studioActiveCompId = c.id;
    showNewCompModal = false;
    editingComponent = null;
    ws.showToast('母版保存成功 ✓', 'success');
  }

  async function handleSaveModule(mod: Module) {
    await api.saveModule(mod);
    await ws.loadModules();
    await ws.loadProfiles();
    activeModuleId = mod.id;
    showModuleMetaModal = false;
    showModuleParamsModal = false;
    editingModule = null;
    tuningModule = null;
    ws.showToast('成品积木已保存 ✓', 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }

  async function handleSaveDevice(d: Device) {
    await api.saveDevice(d);
    await ws.loadDevices();
    showDeviceModal = false;
    editingDevice = null;
    ws.showToast('节点保存成功 ✓', 'success');
  }
</script>

<div class="flex h-screen w-screen overflow-hidden bg-zinc-100 text-zinc-900 transition-colors duration-200 dark:bg-zinc-950 dark:text-zinc-100 font-sans">
  {#if ws.toastMessage}
    <div class="fixed top-5 left-1/2 -translate-x-1/2 z-50 flex items-center gap-2 rounded-full border border-emerald-500/30 bg-zinc-900/90 dark:bg-zinc-900/95 px-3.5 py-1.5 shadow-lg backdrop-blur-md text-xs text-white">
      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 shadow-[0_0_6px_#34d399]"></span>
      <span>{ws.toastMessage.text}</span>
    </div>
  {/if}

  <!-- 左侧全局主侧边栏 -->
  <aside class="flex shrink-0 flex-col justify-between border-r border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950/80 z-20 transition-all duration-200 {ws.isSidebarExpanded ? 'w-44 px-3 py-3.5' : 'w-13 items-center py-3.5'}">
    <div class="flex flex-col gap-5 {ws.isSidebarExpanded ? 'items-start w-full' : 'items-center'}">
      <div class="flex items-center justify-between w-full">
        <div class="flex items-center gap-2.5">
          <div title="Sing-Hub Core" class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-zinc-900 text-white font-mono text-xs font-bold dark:bg-white dark:text-zinc-950 shadow-sm">
            S
          </div>
          {#if ws.isSidebarExpanded}
            <span class="font-mono text-xs font-bold tracking-wider text-zinc-900 dark:text-zinc-100">SING-HUB</span>
          {/if}
        </div>
        <button
          onclick={() => ws.toggleSidebar()}
          class="flex h-6 w-6 items-center justify-center rounded text-zinc-400 hover:bg-zinc-100 hover:text-zinc-800 dark:hover:bg-zinc-800 dark:hover:text-zinc-200 transition cursor-pointer"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            {#if ws.isSidebarExpanded}
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
            {:else}
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 5l7 7-7 7M5 5l7 7-7 7" />
            {/if}
          </svg>
        </button>
      </div>

      <nav class="flex flex-col gap-1.5 w-full">
        <button
          onclick={() => ws.switchWorkspace('studio')}
          class="flex items-center gap-3 rounded-lg text-xs font-medium transition cursor-pointer {ws.isSidebarExpanded ? 'w-full px-3 py-2' : 'h-9 w-9 justify-center'} {ws.currentWorkspace === 'studio' ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 font-semibold' : 'text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
        >
          <Icon name="layers" color={ws.currentWorkspace === 'studio' ? 'amber' : 'default'} size={17} />
          {#if ws.isSidebarExpanded}<span>装配大盘</span>{/if}
        </button>

        <button
          onclick={() => ws.switchWorkspace('modules')}
          class="flex items-center gap-3 rounded-lg text-xs font-medium transition cursor-pointer {ws.isSidebarExpanded ? 'w-full px-3 py-2' : 'h-9 w-9 justify-center'} {ws.currentWorkspace === 'modules' ? 'bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 font-semibold' : 'text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
        >
          <Icon name="boxes" color={ws.currentWorkspace === 'modules' ? 'blue' : 'default'} size={17} />
          {#if ws.isSidebarExpanded}<span>成品积木库</span>{/if}
        </button>

        <button
          onclick={() => {
            ws.switchWorkspace('foundry');
            if (ws.components.length > 0 && !studioActiveCompId) studioActiveCompId = ws.components[0].id;
          }}
          class="flex items-center gap-3 rounded-lg text-xs font-medium transition cursor-pointer {ws.isSidebarExpanded ? 'w-full px-3 py-2' : 'h-9 w-9 justify-center'} {ws.currentWorkspace === 'foundry' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-semibold' : 'text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
        >
          <Icon name="sliders" color={ws.currentWorkspace === 'foundry' ? 'emerald' : 'default'} size={17} />
          {#if ws.isSidebarExpanded}<span>语法母版工坊</span>{/if}
        </button>

        <button
          onclick={() => ws.switchWorkspace('fleet')}
          class="flex items-center gap-3 rounded-lg text-xs font-medium transition cursor-pointer {ws.isSidebarExpanded ? 'w-full px-3 py-2' : 'h-9 w-9 justify-center'} {ws.currentWorkspace === 'fleet' ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 font-semibold' : 'text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
        >
          <Icon name="server" color={ws.currentWorkspace === 'fleet' ? 'blue' : 'default'} size={17} />
          {#if ws.isSidebarExpanded}<span>节点管理</span>{/if}
        </button>
      </nav>
    </div>

    <div class="flex flex-col gap-2 w-full {ws.isSidebarExpanded ? 'items-start' : 'items-center'}">
      <button
        onclick={() => (ws.showSettingsDrawer = true)}
        class="flex items-center gap-3 rounded-lg text-xs text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800 cursor-pointer transition {ws.isSidebarExpanded ? 'w-full px-3 py-2' : 'h-8 w-8 justify-center'}"
      >
        <Icon name="settings" size={17} />
        {#if ws.isSidebarExpanded}<span>系统设置</span>{/if}
      </button>
    </div>
  </aside>

  <!-- 右侧主区域：单层 Header + 动态挂载工作区 -->
  <div class="flex flex-1 min-w-0 flex-col overflow-hidden">
    <!-- 统一全局顶部 Header (不搞任何重复标题) -->
    <header class="flex h-14 shrink-0 items-center justify-between border-b border-zinc-200 bg-white/90 px-6 backdrop-blur-md dark:border-zinc-800 dark:bg-zinc-950/90 z-10">
      <div class="flex items-center gap-3">
        {#if ws.currentWorkspace === 'studio'}
          <Icon name="layers" color="amber" size={20} />
          <div>
            <h1 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 leading-tight">方案装配大盘 (Studio)</h1>
            <span class="text-[10px] text-zinc-400">编排成品积木流水线，输出最终合规全局配置</span>
          </div>
        {:else if ws.currentWorkspace === 'modules'}
          <Icon name="boxes" color="blue" size={20} />
          <div>
            <h1 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 leading-tight">成品积木库 (Modules Inventory)</h1>
            <span class="text-[10px] text-zinc-400">母版派生的独立实参实体，支持同协议多实例与跨方案复用</span>
          </div>
        {:else if ws.currentWorkspace === 'foundry'}
          <Icon name="sliders" color="emerald" size={20} />
          <div>
            <h1 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 leading-tight">原子语法母版工坊 (Foundry)</h1>
            <span class="text-[10px] text-zinc-400">1:1 对标 Sing-box 官方字段与插孔定义，提供软链接指针</span>
          </div>
        {:else}
          <Icon name="server" color="blue" size={20} />
          <div>
            <h1 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 leading-tight">网络节点与实例 (Nodes & Instances)</h1>
            <span class="text-[10px] text-zinc-400">管理基础设施节点，原生支持 IPv6/双栈多端点与四步安全推送</span>
          </div>
        {/if}
      </div>

      <div class="flex items-center gap-2">
        {#if ws.currentWorkspace === 'fleet'}
          <button onclick={() => { isDeviceManageMode = !isDeviceManageMode; selectedDeviceIds = []; }} class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 shadow-2xs hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer flex items-center gap-1.5">
            <Icon name="sliders" size={13} />
            <span>{isDeviceManageMode ? '退出管理' : '批量整理'}</span>
          </button>
          <button disabled={ws.isPingAllLoading} onclick={() => ws.pingAllNodes()} class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 shadow-2xs hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer disabled:opacity-50 flex items-center gap-1.5">
            <Icon name="refresh" size={13} class={ws.isPingAllLoading ? 'animate-spin' : ''} />
            <span>{ws.isPingAllLoading ? '探测中...' : '一键全网探活'}</span>
          </button>
          <button onclick={() => { editingDevice = null; showDeviceModal = true; }} class="rounded-lg bg-blue-600 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 shadow-xs transition cursor-pointer flex items-center gap-1.5">
            <Icon name="plus" size={14} />
            <span>接入新节点</span>
          </button>
        {/if}
      </div>
    </header>

    <!-- 工作区主体 -->
    {#key ws.currentWorkspace}
      {#if ws.currentWorkspace === 'studio'}
        <StudioWorkspace
          bind:isProfileManageMode
          bind:selectedProfileIds
          onNewProfile={() => { editingProfile = null; showProfileModal = true; }}
          onEditProfile={(p) => { editingProfile = p; showProfileModal = true; }}
          onCloneProfile={(source) => {
            editingProfile = { ...source, id: `${source.id}-copy`, name: `${source.name} (副本)`, token: crypto.randomUUID().replace(/-/g, '').slice(0, 16) };
            showProfileModal = true;
          }}
          onOpenMount={() => (showMountDrawer = true)}
          onTuneModuleParams={(mod) => {
            tuningModule = mod;
            showModuleParamsModal = true; // 就地打开纯实参赋值弹窗
          }}
          {triggerConfirm}
        />
      {:else if ws.currentWorkspace === 'modules'}
        <ModulesWorkspace
          bind:activeModuleId
          bind:isModuleManageMode
          bind:selectedModuleIds
          onNewModule={() => { editingModule = null; showModuleMetaModal = true; }}
          onEditModuleMeta={(mod) => { editingModule = mod; showModuleMetaModal = true; }}
          onCloneModule={(source) => {
            editingModule = { ...source, id: `${source.id}-copy`, name: `${source.name} (副本)` };
            showModuleMetaModal = true;
          }}
          {triggerConfirm}
        />
      {:else if ws.currentWorkspace === 'foundry'}
        <FoundryWorkspace
          bind:studioActiveCompId
          bind:isCompManageMode
          bind:selectedCompIds
          onNewComponent={() => { editingComponent = null; showNewCompModal = true; }}
          onEditComponent={(comp) => { editingComponent = comp; showNewCompModal = true; }}
          onCloneComponent={(source) => {
            editingComponent = { ...source, id: `${source.id}-copy`, name: `${source.name} (副本)` };
            showNewCompModal = true;
          }}
          {triggerConfirm}
        />
      {:else if ws.currentWorkspace === 'fleet'}
        <FleetWorkspace
          bind:isDeviceManageMode
          bind:selectedDeviceIds
          onEditDevice={(dev) => { editingDevice = dev; showDeviceModal = true; }}
          onCloneDevice={(source) => {
            editingDevice = { ...source, id: `${source.id}-copy`, name: `${source.name} (副本)` };
            showDeviceModal = true;
          }}
          {triggerConfirm}
        />
      {/if}
    {/key}
  </div>
</div>

<!-- 批量操作浮岛 -->
{#if selectedCompIds.length > 0 || selectedModuleIds.length > 0 || selectedDeviceIds.length > 0 || selectedProfileIds.length > 0}
  <div class="fixed bottom-6 left-1/2 -translate-x-1/2 z-40 flex items-center gap-4 rounded-full border border-zinc-700 bg-zinc-900/95 px-5 py-2.5 shadow-2xl backdrop-blur-md text-xs text-white">
    {#if selectedCompIds.length > 0}
      <span>已选 <strong>{selectedCompIds.length}</strong> 个母版</span>
      <button onclick={() => triggerConfirm('确认批量删除？', `即将永久删除已选的 ${selectedCompIds.length} 个母版。`, () => { ws.batchDeleteComponents(selectedCompIds); selectedCompIds = []; isCompManageMode = false; })} class="rounded bg-rose-600 px-3 py-1 font-semibold hover:bg-rose-500 cursor-pointer">批量删除</button>
      <button onclick={() => { selectedCompIds = []; isCompManageMode = false; }} class="text-zinc-400 hover:text-white cursor-pointer">取消</button>
    {:else if selectedModuleIds.length > 0}
      <span>已选 <strong>{selectedModuleIds.length}</strong> 个成品积木</span>
      <button onclick={() => triggerConfirm('确认批量删除？', `即将永久删除已选的 ${selectedModuleIds.length} 个成品积木。`, () => { ws.batchDeleteModules(selectedModuleIds); selectedModuleIds = []; isModuleManageMode = false; })} class="rounded bg-rose-600 px-3 py-1 font-semibold hover:bg-rose-500 cursor-pointer">批量删除</button>
      <button onclick={() => { selectedModuleIds = []; isModuleManageMode = false; }} class="text-zinc-400 hover:text-white cursor-pointer">取消</button>
    {:else if selectedDeviceIds.length > 0}
      <span>已选 <strong>{selectedDeviceIds.length}</strong> 个节点</span>
      <button onclick={() => triggerConfirm('确认批量移除？', `即将永久移除已选的 ${selectedDeviceIds.length} 个节点。`, () => { ws.batchDeleteDevices(selectedDeviceIds); selectedDeviceIds = []; isDeviceManageMode = false; })} class="rounded bg-rose-600 px-3 py-1 font-semibold hover:bg-rose-500 cursor-pointer">批量删除</button>
      <button onclick={() => { selectedDeviceIds = []; isDeviceManageMode = false; }} class="text-zinc-400 hover:text-white cursor-pointer">取消</button>
    {:else if selectedProfileIds.length > 0}
      <span>已选 <strong>{selectedProfileIds.length}</strong> 个方案配方</span>
      <button onclick={() => triggerConfirm('确认批量删除？', `即将永久删除已选的 ${selectedProfileIds.length} 个方案。`, () => { ws.batchDeleteProfiles(selectedProfileIds); selectedProfileIds = []; isProfileManageMode = false; })} class="rounded bg-rose-600 px-3 py-1 font-semibold hover:bg-rose-500 cursor-pointer">批量删除</button>
      <button onclick={() => { selectedProfileIds = []; isProfileManageMode = false; }} class="text-zinc-400 hover:text-white cursor-pointer">取消</button>
    {/if}
  </div>
{/if}

<!-- 抽屉与模态框组 -->
<MountDrawer open={showMountDrawer} onClose={async () => { showMountDrawer = false; await ws.loadProfiles(); }} />
<SettingsDrawer open={ws.showSettingsDrawer} onClose={() => (ws.showSettingsDrawer = false)} />
<ComponentModal open={showNewCompModal} editComponent={editingComponent} onClose={() => { showNewCompModal = false; editingComponent = null; }} onSave={handleSaveComponent} />
<ModuleMetaModal open={showModuleMetaModal} editModule={editingModule} onClose={() => { showModuleMetaModal = false; editingModule = null; }} onSave={handleSaveModule} />
<ModuleParamsModal open={showModuleParamsModal} module={tuningModule} onClose={() => { showModuleParamsModal = false; tuningModule = null; }} onSave={handleSaveModule} />
<DeviceModal open={showDeviceModal} editDevice={editingDevice} onClose={() => (showDeviceModal = false)} onSave={handleSaveDevice} />
<ProfileModal open={showProfileModal} editProfile={editingProfile} onClose={() => (showProfileModal = false)} onSave={handleSaveProfile} />

<ConfirmModal
  open={confirmState.open}
  title={confirmState.title}
  content={confirmState.content}
  actionType={confirmState.actionType}
  onConfirm={confirmState.onConfirm}
  onCancel={() => (confirmState.open = false)}
/>