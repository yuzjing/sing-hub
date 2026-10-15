<!-- frontend/src/routes/+page.svelte -->
<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Component, Module, Profile, Device } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';

  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import MountDrawer from '$lib/components/MountDrawer.svelte';
  import SettingsDrawer from '$lib/components/SettingsDrawer.svelte';
  import ComponentModal from '$lib/components/ComponentModal.svelte';
  import ModuleModal from '$lib/components/ModuleModal.svelte';
  import DeviceModal from '$lib/components/DeviceModal.svelte';
  import ProfileModal from '$lib/components/ProfileModal.svelte';

  let copiedSub = $state(false);
  let copiedJSON = $state(false);
  let isDeploying = $state(false);

  let showMountDrawer = $state(false);

  // 方案配方 (PROFILES) 管理模式
  let isProfileManageMode = $state(false);
  let selectedProfileIds = $state<string[]>([]);
  let editingProfile = $state<Profile | null>(null);

  // 成品积木 (MODULES) 管理模式
  let isModuleManageMode = $state(false);
  let selectedModuleIds = $state<string[]>([]);
  let editingModule = $state<Module | null>(null);
  let showModuleModal = $state(false);

  // 语法母版 (FOUNDRY) 管理模式
  let isCompManageMode = $state(false);
  let selectedCompIds = $state<string[]>([]);
  let showNewCompModal = $state(false);

  // 节点运维 (FLEET) 管理模式
  let editingDevice = $state<Device | null>(null);
  let isDeviceManageMode = $state(false);
  let selectedDeviceIds = $state<string[]>([]);
  let showDeviceModal = $state(false);

  let confirmState = $state({
    open: false,
    title: '',
    content: '',
    actionType: 'danger' as 'danger' | 'primary',
    onConfirm: () => {},
  });

  let showProfileModal = $state(false);

  // 工坊沙箱编辑器状态
  let studioActiveCompId = $state<string>('');
  let studioTmplCode = $state<string>('');
  let studioPreviewJSON = $state<string>('// 选择积木母版查看实时语法渲染');
  let studioPreviewError = $state<string>('');
  let showContextVarMenu = $state(false);

  let editorRef = $state<{ insertAtCursor: (snippet: string) => void } | null>(null);

  let isTagDropdownOpen = $state(false);
  let selectedFilterTags = $state<string[]>([]);

  let availableTags = $derived.by(() => {
    const tags: string[] = [];
    ws.components.forEach((c: Component) => {
      if (c.scope_tag) {
        c.scope_tag.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean).forEach(t => {
          if (!tags.includes(t)) tags.push(t);
        });
      }
    });
    return tags;
  });

  function extractAllTemplateVars(code: string): string[] {
    const matches = code.matchAll(/\{\{\s*\.([a-zA-Z0-9_.]+)(?:\s*\|[^}]+)?\s*\}\}/g);
    const uniqueVars: string[] = [];
    for (const m of matches) {
      const v = m[1];
      if (!uniqueVars.includes(v)) {
        uniqueVars.push(v);
      }
    }
    return uniqueVars;
  }

  function safeParseJson(raw: string | undefined): Record<string, unknown> {
    if (!raw) return {};
    try {
      return JSON.parse(raw);
    } catch {
      return {};
    }
  }

  let studioDetectedVars = $derived.by(() => {
    return extractAllTemplateVars(studioTmplCode);
  });

  let studioTestParams = $state<Record<string, string | number | boolean>>({});

  $effect(() => {
    const vars = studioDetectedVars;
    untrack(() => {
      let changed = false;
      const next: Record<string, string | number | boolean> = { ...studioTestParams };
      vars.forEach(v => {
        if (next[v] === undefined) {
          changed = true;
          if (v.includes('port')) next[v] = 443;
          else if (v.includes('uuid')) next[v] = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890';
          else if (v.includes('endpoint') || v.includes('host') || v.includes('server')) next[v] = '10.123.123.1';
          else next[v] = 'test_value';
        }
      });
      if (changed) {
        studioTestParams = next;
      }
    });
  });

  const CATEGORIES = [
    { key: 'io', title: '流量出入 (IO)', color: 'blue' as const },
    { key: 'routing', title: '路由决策 (Routing)', color: 'amber' as const },
    { key: 'dns', title: '域名解析 (DNS)', color: 'emerald' as const },
    { key: 'system', title: '系统基础设施 (System)', color: 'purple' as const }
  ];

  onMount(() => {
    ws.init();
  });

  async function copySubURL(token: string) {
    const url = `${window.location.origin}/sub/${token}`;
    await navigator.clipboard.writeText(url);
    copiedSub = true;
    ws.showToast('订阅链接已复制 ✓', 'success');
    setTimeout(() => (copiedSub = false), 1400);
  }

  async function copyCompiledJSON() {
    if (!ws.compiledJSON) return;
    await navigator.clipboard.writeText(ws.compiledJSON);
    copiedJSON = true;
    ws.showToast('完整配置 JSON 已复制 ✓', 'success');
    setTimeout(() => (copiedJSON = false), 1400);
  }

  async function selectStudioComp(c: Component) {
    studioActiveCompId = c.id;
    studioTmplCode = c.payload_tmpl;
    runStudioPreview();
  }

  async function runStudioPreview() {
    studioPreviewError = '';
    try {
      const nestedParams: Record<string, unknown> = {};
      for (const [key, val] of Object.entries(studioTestParams)) {
        if (key.includes('.')) {
          const parts = key.split('.');
          let cur: Record<string, unknown> = nestedParams;
          for (let i = 0; i < parts.length - 1; i++) {
            if (!cur[parts[i]]) cur[parts[i]] = {};
            cur = cur[parts[i]] as Record<string, unknown>;
          }
          cur[parts[parts.length - 1]] = val;
        } else {
          nestedParams[key] = val;
        }
      }

      studioPreviewJSON = await api.previewComponent(studioTmplCode, JSON.stringify(nestedParams), '');
    } catch (e: unknown) {
      studioPreviewError = e instanceof Error ? e.message : '渲染错误';
    }
  }

  function handleTestParamInput(key: string, rawVal: string) {
    let parsed: string | number | boolean = rawVal;
    if (rawVal.trim() !== '' && !isNaN(Number(rawVal))) {
      parsed = Number(rawVal);
    } else if (rawVal.toLowerCase() === 'true') {
      parsed = true;
    } else if (rawVal.toLowerCase() === 'false') {
      parsed = false;
    }
    studioTestParams[key] = parsed;
    runStudioPreview();
  }

  function handleInsertVar(snippet: string) {
    if (editorRef) {
      editorRef.insertAtCursor(snippet);
    } else {
      studioTmplCode += snippet;
    }
    showContextVarMenu = false;
    runStudioPreview();
  }

  async function saveStudioComp() {
    const c = ws.components.find((item: Component) => item.id === studioActiveCompId);
    if (!c) return;
    c.payload_tmpl = studioTmplCode;
    await api.saveComponent(c);
    await ws.loadComponents();
    await ws.loadProfiles();
    ws.showToast('母版已保存并提交 Git ✓', 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }

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
    ws.showToast(editingProfile ? '方案更新成功 ✓' : '方案创建成功 ✓', 'success');
  }

  function cloneProfile(source: Profile) {
    editingProfile = {
      ...source,
      id: `${source.id}-copy`,
      name: `${source.name} (副本)`,
      token: crypto.randomUUID().replace(/-/g, '').slice(0, 16),
      sing_box_version: source.sing_box_version,
      modules: source.modules || [],
    };
    showProfileModal = true;
  }

  async function handleCreateComponent(comp: { name: string; category: string; scope_tag: string; mount_path: string; strategy: 'append'|'merge'; payload_tmpl: string }) {
    const c: Component = {
      id: 'comp-' + Date.now().toString(36),
      name: comp.name,
      category: comp.category,
      scope_tag: comp.scope_tag,
      mount_path: comp.mount_path,
      strategy: comp.strategy,
      payload_tmpl: comp.payload_tmpl,
      description: ''
    };
    await api.saveComponent(c);
    await ws.loadComponents();
    selectStudioComp(c);
    showNewCompModal = false;
    ws.showToast('母版已入库 ✓', 'success');
  }

  async function handleSaveModule(mod: Module) {
    await api.saveModule(mod);
    await ws.loadModules();
    await ws.loadProfiles();
    showModuleModal = false;
    editingModule = null;
    ws.showToast('成品积木已保存 ✓', 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }

  async function handleSaveDevice(d: Device) {
    await api.saveDevice(d);
    await ws.loadDevices();
    showDeviceModal = false;
    editingDevice = null;
    ws.showToast(editingDevice ? '节点更新成功 ✓' : '节点接入成功 ✓', 'success');
  }

  function cloneDevice(source: Device) {
    editingDevice = {
      ...source,
      id: `${source.id}-copy`,
      name: `${source.name} (副本)`,
    };
    showDeviceModal = true;
  }

  async function deployCurrentDevice(deviceId: string) {
    isDeploying = true;
    try {
      const res = await api.deployDevice(deviceId);
      ws.showToast(`部署成功！远程哈希已严格校验: ${res.verified_hash} ✓`, 'success');
    } catch (e: unknown) {
      ws.showToast(e instanceof Error ? e.message : '部署失败', 'error');
    } finally {
      isDeploying = false;
    }
  }

  function triggerConfirm(title: string, content: string, onConfirm: () => void, actionType: 'danger' | 'primary' = 'danger') {
    confirmState = { open: true, title, content, onConfirm, actionType };
  }

  function toggleTagFilter(tag: string) {
    if (selectedFilterTags.includes(tag)) {
      selectedFilterTags = selectedFilterTags.filter(t => t !== tag);
    } else {
      selectedFilterTags = [...selectedFilterTags, tag];
    }
  }
</script>

<div class="flex h-screen w-screen overflow-hidden bg-zinc-100 text-zinc-900 transition-colors duration-200 dark:bg-zinc-950 dark:text-zinc-100 font-sans">
  {#if ws.toastMessage}
    <div class="fixed top-5 left-1/2 -translate-x-1/2 z-50 flex items-center gap-2 rounded-full border border-emerald-500/30 bg-zinc-900/90 dark:bg-zinc-900/95 px-3.5 py-1.5 shadow-lg backdrop-blur-md text-xs text-white">
      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 shadow-[0_0_6px_#34d399]"></span>
      <span>{ws.toastMessage.text}</span>
    </div>
  {/if}

  <!-- 左侧主侧边栏 -->
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
            if (ws.components.length > 0 && !studioActiveCompId) selectStudioComp(ws.components[0]);
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

  <div class="flex flex-1 min-w-0 flex-col overflow-hidden">
    <!-- 顶部状态栏 -->
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
            <span class="text-[10px] text-zinc-400">由母版派生独立实参成品，支持多出站、多分流实例复用</span>
          </div>
        {:else if ws.currentWorkspace === 'foundry'}
          <Icon name="sliders" color="emerald" size={20} />
          <div>
            <h1 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 leading-tight">原子语法母版工坊 (Foundry)</h1>
            <span class="text-[10px] text-zinc-400">1:1 对标 Sing-box 官方字段模板与插孔定义，提供软链接指针</span>
          </div>
        {:else}
          <Icon name="server" color="blue" size={20} />
          <div>
            <h1 class="text-sm font-bold text-zinc-900 dark:text-zinc-100 leading-tight">网络节点与实例 (Nodes & Instances)</h1>
            <span class="text-[10px] text-zinc-400">管理基础设施节点，原生支持 IPv6/双栈多端点与四步安全推送</span>
          </div>
        {/if}
      </div>

      <div class="flex items-center gap-2.5">
        {#if ws.currentWorkspace === 'fleet'}
          <button onclick={() => { isDeviceManageMode = !isDeviceManageMode; selectedDeviceIds = []; }} class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 shadow-sm hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer flex items-center gap-1.5">
            <Icon name="sliders" size={13} />
            <span>{isDeviceManageMode ? '退出管理' : '批量整理'}</span>
          </button>
          <button disabled={ws.isPingAllLoading} onclick={() => ws.pingAllNodes()} class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 shadow-sm hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer disabled:opacity-50 flex items-center gap-1.5">
            <Icon name="refresh" size={13} class={ws.isPingAllLoading ? 'animate-spin' : ''} />
            <span>{ws.isPingAllLoading ? '探测中...' : '一键全网探活'}</span>
          </button>
          <button onclick={() => { editingDevice = null; showDeviceModal = true; }} class="rounded-lg bg-blue-600 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 shadow-sm transition cursor-pointer flex items-center gap-1.5">
            <Icon name="plus" size={14} />
            <span>接入新节点</span>
          </button>
        {/if}
      </div>
    </header>

    {#key ws.currentWorkspace}
      <div class="flex flex-1 min-h-0 overflow-hidden relative">

        <!-- ==================== 工作区 1: 装配大盘 (Studio) ==================== -->
        {#if ws.currentWorkspace === 'studio'}
          <aside class="flex w-64 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50/70 dark:border-zinc-800 dark:bg-zinc-950/40">
            <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-3.5 dark:border-zinc-800">
              <span class="text-[11px] font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500">方案配方 (PROFILES)</span>
              <div class="flex items-center gap-1">
                <button
                  title={isProfileManageMode ? '退出管理' : '批量整理方案'}
                  onclick={() => { isProfileManageMode = !isProfileManageMode; selectedProfileIds = []; }}
                  class="flex h-6 w-6 items-center justify-center rounded text-xs transition cursor-pointer {isProfileManageMode ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300 font-bold' : 'text-zinc-400 hover:bg-zinc-200/50 hover:text-zinc-700 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
                >
                  <Icon name="sliders" size={14} />
                </button>
                <button
                  title="新建方案配方"
                  onclick={() => { editingProfile = null; showProfileModal = true; }}
                  class="flex h-6 w-6 items-center justify-center rounded text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/50 cursor-pointer"
                >
                  <Icon name="plus" size={15} />
                </button>
              </div>
            </div>

            <div class="flex-1 overflow-y-auto p-2 space-y-1">
              {#each ws.profiles as p (p.id)}
                <div class="group flex items-center justify-between rounded-lg px-2.5 py-2 text-xs transition {ws.activeProfileId === p.id && !isProfileManageMode ? 'bg-white shadow-sm ring-1 ring-zinc-300 text-zinc-900 font-semibold dark:bg-zinc-800/90 dark:ring-zinc-700 dark:text-white' : 'text-zinc-600 hover:bg-zinc-200/60 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
                  <div class="flex items-center gap-2 truncate flex-1">
                    {#if isProfileManageMode}
                      <input
                        type="checkbox"
                        checked={selectedProfileIds.includes(p.id)}
                        onchange={(e) => {
                          if ((e.target as HTMLInputElement).checked) selectedProfileIds = [...selectedProfileIds, p.id];
                          else selectedProfileIds = selectedProfileIds.filter(id => id !== p.id);
                        }}
                        class="rounded"
                      />
                    {/if}
                    <button onclick={() => ws.selectProfile(p.id)} class="truncate text-left flex-1 cursor-pointer">
                      {p.name}
                    </button>
                  </div>

                  <div class="flex items-center gap-1.5">
                    {#if !isProfileManageMode}
                      {#if p.capabilities.enable_subscribe}
                        <button
                          onclick={() => copySubURL(p.token)}
                          title="点击直接复制订阅链接"
                          class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5"
                        >
                          <Icon name="link" size={13} />
                        </button>
                      {/if}
                    {:else}
                      <button title="克隆方案" onclick={() => cloneProfile(p)} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                        <Icon name="copy" size={13} />
                      </button>
                      <button title="编辑元数据" onclick={() => { editingProfile = p; showProfileModal = true; }} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                        <Icon name="pencil" size={13} />
                      </button>
                      <button title="删除方案" onclick={() => triggerConfirm('确认删除方案？', `即将永久删除方案 [${p.name}]。`, () => ws.batchDeleteProfiles([p.id]))} class="text-zinc-400 hover:text-rose-500 cursor-pointer p-0.5">
                        <Icon name="trash" size={13} />
                      </button>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          </aside>

          <!-- 中间装配流水线 -->
          <main class="flex flex-1 min-w-0 flex-col overflow-hidden border-r border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
            <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-5 dark:border-zinc-800">
              <div class="flex items-center gap-2">
                <span class="text-xs text-zinc-400">当前方案:</span>
                <strong class="text-xs text-zinc-900 dark:text-zinc-100">{ws.activeProfile?.name || '请选择方案'}</strong>
                {#if ws.activeProfile?.sing_box_version}
                  <span class="rounded bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500">{ws.activeProfile.sing_box_version}</span>
                {/if}
              </div>

              {#if ws.activeProfile}
                <button
                  onclick={() => (showMountDrawer = true)}
                  class="rounded-md bg-blue-600 px-3 py-1 text-xs font-semibold text-white shadow-sm hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5"
                >
                  <Icon name="plus" size={13} />
                  <span>装配成品积木</span>
                </button>
              {/if}
            </div>

            <div class="flex-1 overflow-y-auto p-5 space-y-3">
              {#if ws.resolvedModuleBindings.length === 0}
                <div class="flex h-80 flex-col items-center justify-center rounded-xl border border-dashed border-zinc-300 p-8 text-center dark:border-zinc-800">
                  <span class="text-xs text-zinc-400">当前方案暂未装配成品积木，点击右上角从积木库中挑选</span>
                </div>
              {:else}
                {#each ws.resolvedModuleBindings as item, idx (item.binding.id ?? idx)}
                  {@const currentParams = safeParseJson(item.module.params_json)}
                  <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-4 shadow-sm hover:border-zinc-300 dark:border-zinc-800/90 dark:bg-zinc-900/60 dark:hover:border-zinc-700 transition">
                    <div class="flex items-center justify-between">
                      <div class="flex items-center gap-3">
                        <span class="font-mono text-xs font-bold text-zinc-400 dark:text-zinc-600">0{idx + 1}</span>
                        <span class="text-xs font-semibold text-zinc-900 dark:text-zinc-100">{item.module.name}</span>
                        <span class="rounded bg-zinc-200/80 px-2 py-0.5 font-mono text-[10px] text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                          {item.template?.mount_path || '自定义路径'}
                        </span>
                        <span class="text-[10px] text-zinc-400 font-mono">
                          母版: {item.template?.name || item.module.template_id}
                        </span>
                      </div>

                      <div class="flex items-center gap-1.5">
                        <button
                          onclick={() => {
                            editingModule = item.module;
                            showModuleModal = true;
                          }}
                          class="rounded border border-zinc-200 bg-white px-2 py-1 text-xs text-zinc-600 hover:border-zinc-300 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 cursor-pointer"
                        >
                          编辑实参
                        </button>
                        <button disabled={idx === 0} onclick={() => ws.moveModuleBinding(idx, 'up')} class="rounded px-2 py-1 text-xs text-zinc-500 hover:bg-zinc-200 disabled:opacity-30 dark:hover:bg-zinc-800 cursor-pointer">↑</button>
                        <button disabled={idx === ws.resolvedModuleBindings.length - 1} onclick={() => ws.moveModuleBinding(idx, 'down')} class="rounded px-2 py-1 text-xs text-zinc-500 hover:bg-zinc-200 disabled:opacity-30 dark:hover:bg-zinc-800 cursor-pointer">↓</button>
                        <button onclick={() => ws.detachModule(item.binding.id ?? item.module.id)} class="rounded p-1 text-zinc-400 hover:text-rose-500 cursor-pointer">
                          <Icon name="close" size={13} />
                        </button>
                      </div>
                    </div>

                    <div class="mt-2.5 pt-2 border-t border-zinc-200/60 dark:border-zinc-800/60 flex flex-wrap gap-2 text-[11px] font-mono">
                      {#each Object.entries(currentParams) as [k, val] (k)}
                        <span class="rounded bg-white dark:bg-zinc-950 px-2 py-0.5 border border-zinc-200 dark:border-zinc-800 text-zinc-600 dark:text-zinc-400">
                          .{k}: <strong class="text-zinc-800 dark:text-zinc-200">{String(val)}</strong>
                        </span>
                      {:else}
                        <span class="text-zinc-400 text-[10px] italic">该成品使用纯静态或系统环境上下文。</span>
                      {/each}
                    </div>
                  </div>
                {/each}
              {/if}
            </div>
          </main>

          <!-- 右侧编译镜像 -->
          <section class="flex w-[42%] min-w-95 max-w-155 shrink-0 flex-col bg-zinc-50 dark:bg-zinc-950/90">
            <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
              <div class="flex items-center gap-2">
                <span class="font-mono text-xs font-bold text-zinc-500 dark:text-zinc-400">CONFIG MIRROR</span>
                {#if ws.activeCommitHash}
                  <button onclick={() => ws.openTimeline()} class="flex items-center gap-1 rounded bg-zinc-200/80 px-2 py-0.5 font-mono text-[10px] font-semibold text-zinc-700 hover:bg-zinc-300 dark:bg-zinc-800 dark:text-zinc-300 cursor-pointer">
                    <span>git: {ws.activeCommitHash}</span>
                  </button>
                {/if}
              </div>
              {#if ws.compileError}
                <span class="text-[11px] text-rose-500 font-mono font-bold flex items-center gap-1 max-w-45 truncate" title={ws.compileError}>
                  <Icon name="close" size={12} />
                  <span>语法校验未通过</span>
                </span>
              {:else}
                <span class="text-[10px] text-emerald-500 font-semibold flex items-center gap-1">
                  <Icon name="check" size={12} />
                  <span>语法校验通过</span>
                </span>
              {/if}
            </div>

            {#if ws.compileError}
              <div class="m-3 p-3 rounded-lg bg-rose-50 text-rose-800 border border-rose-200 dark:bg-rose-950/60 dark:text-rose-200 dark:border-rose-800 text-[11px] font-mono whitespace-pre-wrap overflow-y-auto max-h-48 shadow-inner">
                {ws.compileError}
              </div>
            {/if}

            <div class="flex-1 overflow-hidden p-3 pt-0">
              <div class="h-full rounded-xl border border-zinc-200 bg-white p-1 shadow-sm dark:border-zinc-800/80 dark:bg-zinc-950">
                <CodeEditor value={ws.compiledJSON} readonly={true} dark={ws.isDark} />
              </div>
            </div>

            {#if ws.activeProfile}
              <div class="flex items-center justify-between border-t border-zinc-200 bg-white/90 p-3 dark:border-zinc-800 dark:bg-zinc-950">
                <div class="flex items-center gap-2">
                  <button
                    onclick={copyCompiledJSON}
                    class="rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs font-mono font-medium text-zinc-700 shadow-sm hover:border-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1.5"
                  >
                    <Icon name="copy" size={13} />
                    <span>{copiedJSON ? '已复制' : '复制 JSON'}</span>
                  </button>

                  {#if ws.activeProfile.capabilities.enable_subscribe}
                    <button
                      onclick={() => copySubURL(ws.activeProfile?.token ?? '')}
                      class="rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-xs font-mono font-medium text-zinc-700 shadow-sm hover:border-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1.5"
                    >
                      <Icon name="link" size={13} />
                      <span>{copiedSub ? '已复制' : '复制订阅'}</span>
                    </button>
                  {/if}
                </div>

                <button
                  onclick={() => ws.triggerCompile()}
                  class="rounded-lg bg-blue-600 px-4 py-1.5 text-xs font-semibold text-white shadow-sm hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5"
                >
                  <span>{ws.isCompiling ? '保存中...' : '💾 保存方案并编译'}</span>
                </button>
              </div>
            {/if}
          </section>

        <!-- ==================== 工作区 2: 成品积木库 (Modules) ==================== -->
        {:else if ws.currentWorkspace === 'modules'}
          <div class="flex-1 overflow-y-auto p-8 max-w-6xl mx-auto w-full space-y-6">
            <div class="flex items-center justify-between border-b border-zinc-200 pb-4 dark:border-zinc-800">
              <div>
                <h2 class="text-lg font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
                  <Icon name="boxes" color="blue" size={19} />
                  <span>成品积木库 (Modules Inventory)</span>
                </h2>
                <p class="text-xs text-zinc-400 mt-1">每个积木引用工坊母版并赋予实参。大盘可无限制重复装配不同积木（如多个出站节点）。</p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  onclick={() => { isModuleManageMode = !isModuleManageMode; selectedModuleIds = []; }}
                  class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 shadow-sm hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer flex items-center gap-1.5"
                >
                  <Icon name="sliders" size={13} />
                  <span>{isModuleManageMode ? '退出管理' : '批量整理'}</span>
                </button>
                <button
                  onclick={() => { editingModule = null; showModuleModal = true; }}
                  class="rounded-lg bg-blue-600 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 shadow-sm transition cursor-pointer flex items-center gap-1.5"
                >
                  <Icon name="plus" size={14} />
                  <span>新建成品积木</span>
                </button>
              </div>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {#each ws.modules as mod (mod.id)}
                {@const pMap = safeParseJson(mod.params_json)}
                <div class="rounded-2xl border border-zinc-200 bg-white p-4 shadow-sm hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-700 transition flex flex-col justify-between space-y-3">
                  <div>
                    <div class="flex items-start justify-between">
                      <div class="flex items-center gap-2">
                        {#if isModuleManageMode}
                          <input
                            type="checkbox"
                            checked={selectedModuleIds.includes(mod.id)}
                            onchange={(e) => {
                              if ((e.target as HTMLInputElement).checked) selectedModuleIds = [...selectedModuleIds, mod.id];
                              else selectedModuleIds = selectedModuleIds.filter(id => id !== mod.id);
                            }}
                            class="rounded"
                          />
                        {/if}
                        <div>
                          <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100">{mod.name}</h3>
                          <span class="text-[10px] text-zinc-400 font-mono">母版: {mod.template?.name || mod.template_id}</span>
                        </div>
                      </div>
                      <span class="rounded bg-zinc-100 px-2 py-0.5 text-[10px] font-mono text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                        {mod.category}
                      </span>
                    </div>

                    <div class="mt-2 text-[11px] text-zinc-500 font-mono flex items-center gap-1">
                      <span class="text-zinc-400">挂载点:</span>
                      <strong class="text-zinc-700 dark:text-zinc-300">{mod.template?.mount_path || '未指定'}</strong>
                    </div>

                    <div class="mt-2 p-2 rounded-lg bg-zinc-50 dark:bg-zinc-950/60 border border-zinc-100 dark:border-zinc-800/80 space-y-1 text-[10px] font-mono">
                      {#each Object.entries(pMap) as [key, val] (key)}
                        <div class="truncate text-zinc-600 dark:text-zinc-400">
                          .{key} = <span class="text-blue-600 dark:text-blue-400">{String(val)}</span>
                        </div>
                      {:else}
                        <div class="text-zinc-400 italic">纯静态，无需实参</div>
                      {/each}
                    </div>
                  </div>

                  <div class="flex items-center justify-end gap-1.5 border-t border-zinc-100 pt-2 dark:border-zinc-800">
                    <button
                      onclick={() => { editingModule = mod; showModuleModal = true; }}
                      class="p-1 rounded text-zinc-400 hover:text-blue-500 cursor-pointer"
                      title="编辑实参"
                    >
                      <Icon name="pencil" size={13} />
                    </button>
                    <button
                      onclick={() => triggerConfirm('删除成品积木？', `将永久删除积木 [${mod.name}]。方案引用也将解除。`, () => ws.batchDeleteModules([mod.id]))}
                      class="p-1 rounded text-zinc-400 hover:text-rose-500 cursor-pointer"
                      title="删除积木"
                    >
                      <Icon name="trash" size={13} />
                    </button>
                  </div>
                </div>
              {/each}
            </div>
          </div>

        <!-- ==================== 工作区 3: 语法母版工坊 (Foundry) ==================== -->
        {:else if ws.currentWorkspace === 'foundry'}
          <aside class="flex w-64 shrink-0 flex-col border-r border-zinc-200 bg-zinc-50/70 dark:border-zinc-800 dark:bg-zinc-950/40">
            <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-3.5 dark:border-zinc-800">
              <span class="text-[11px] font-bold uppercase tracking-wider text-zinc-400 dark:text-zinc-500">语法母版库 (TEMPLATES)</span>
              <div class="flex items-center gap-1">
                <button
                  title={isCompManageMode ? '退出管理' : '批量整理母版'}
                  onclick={() => { isCompManageMode = !isCompManageMode; selectedCompIds = []; }}
                  class="flex h-6 w-6 items-center justify-center rounded text-xs transition cursor-pointer {isCompManageMode ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300 font-bold' : 'text-zinc-400 hover:bg-zinc-200/50 hover:text-zinc-700 dark:hover:bg-zinc-800 dark:hover:text-zinc-200'}"
                >
                  <Icon name="sliders" size={14} />
                </button>
                <button
                  title="新建语法母版"
                  onclick={() => { showNewCompModal = true; }}
                  class="flex h-6 w-6 items-center justify-center rounded text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/50 cursor-pointer"
                >
                  <Icon name="plus" size={15} />
                </button>
              </div>
            </div>

            <div class="relative border-b border-zinc-200 px-3 py-2 dark:border-zinc-800">
              <div class="flex items-center justify-between">
                <span class="text-[10px] font-mono text-zinc-400">标签多选筛选:</span>
                <button onclick={() => (isTagDropdownOpen = !isTagDropdownOpen)} class="rounded border border-zinc-200 bg-white px-2 py-0.5 text-[10px] font-mono text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer">
                  {selectedFilterTags.length === 0 ? '全部显示' : `已选 ${selectedFilterTags.length} 个`} ▾
                </button>
              </div>

              {#if isTagDropdownOpen}
                <div class="absolute left-3 right-3 top-10 z-30 rounded-lg border border-zinc-200 bg-white p-2 shadow-xl dark:border-zinc-700 dark:bg-zinc-900 space-y-1 text-xs">
                  {#if availableTags.length === 0}
                    <span class="text-[10px] text-zinc-400 block px-1">暂无标签</span>
                  {:else}
                    {#each availableTags as t (t)}
                      <label class="flex items-center gap-2 px-1.5 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer font-mono text-[11px]">
                        <input type="checkbox" checked={selectedFilterTags.includes(t)} onchange={() => toggleTagFilter(t)} class="rounded" />
                        <span>{t}</span>
                      </label>
                    {/each}
                    <div class="border-t border-zinc-100 pt-1 flex justify-between items-center text-[10px] text-zinc-400">
                      <button onclick={() => (selectedFilterTags = [])} class="hover:underline">清除</button>
                      <button onclick={() => (isTagDropdownOpen = false)} class="font-bold text-blue-500">完成</button>
                    </div>
                  {/if}
                </div>
              {/if}
            </div>

            <div class="flex-1 overflow-y-auto p-2 space-y-4">
              {#each CATEGORIES as cat (cat.key)}
                <div>
                  <span class="text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider px-2 block mb-1">
                    {cat.title}
                  </span>
                  <div class="flex flex-col gap-1">
                    {#each ws.components.filter(c => c.category === cat.key && (selectedFilterTags.length === 0 || selectedFilterTags.some(t => (c.scope_tag || '').includes(t)))) as comp (comp.id)}
                      <div class="group flex items-center justify-between rounded-lg px-2.5 py-2 text-xs transition {studioActiveCompId === comp.id && !isCompManageMode ? 'bg-white shadow-sm ring-1 ring-zinc-300 text-zinc-900 font-semibold dark:bg-zinc-800 dark:text-white' : 'text-zinc-600 hover:bg-zinc-200/50 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
                        <div class="flex items-center gap-2 truncate flex-1">
                          {#if isCompManageMode}
                            <input
                              type="checkbox"
                              checked={selectedCompIds.includes(comp.id)}
                              onchange={(e) => {
                                if ((e.target as HTMLInputElement).checked) selectedCompIds = [...selectedCompIds, comp.id];
                                else selectedCompIds = selectedCompIds.filter(id => id !== comp.id);
                              }}
                              class="rounded"
                            />
                          {/if}
                          <button onclick={() => selectStudioComp(comp)} class="truncate text-left flex-1 cursor-pointer">
                            {comp.name}
                          </button>
                        </div>

                        <div class="flex items-center gap-1.5">
                          {#if !isCompManageMode}
                            <span class="font-mono text-[9px] text-zinc-400">[{comp.scope_tag}]</span>
                          {:else}
                            <button
                              title="删除母版"
                              onclick={() => triggerConfirm('确认删除母版？', `将永久删除母版 [${comp.name}]。所有引用此母版的成品积木也会被级联删除。`, () => ws.batchDeleteComponents([comp.id]))}
                              class="text-zinc-400 hover:text-rose-500 cursor-pointer p-0.5"
                            >
                              <Icon name="trash" size={13} />
                            </button>
                          {/if}
                        </div>
                      </div>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          </aside>

          <!-- 右侧工坊编辑器与沙箱 -->
          <main class="flex flex-1 min-w-0 overflow-hidden bg-white dark:bg-zinc-950">
            <div class="flex flex-1 flex-col border-r border-zinc-200 dark:border-zinc-800">
              <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
                <span class="font-mono text-xs font-semibold text-zinc-600 dark:text-zinc-400">TEMPLATE SOURCE</span>
                <div class="flex items-center gap-2 relative">
                  <button
                    onclick={() => (showContextVarMenu = !showContextVarMenu)}
                    class="rounded border border-zinc-300 bg-zinc-50 px-2.5 py-1 text-xs text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1"
                  >
                    <span>➕ 插入变量 ▾</span>
                  </button>

                  {#if showContextVarMenu}
                    <div class="absolute right-24 top-10 z-40 w-64 rounded-xl border border-zinc-200 bg-white p-2.5 shadow-2xl dark:border-zinc-800 dark:bg-zinc-900 text-xs space-y-2.5">
                      <div>
                        <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1 mb-1">节点网络上下文 (NODE)</span>
                        <button onclick={() => handleInsertVar('{{ ._node.endpoint | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block">._node.endpoint (带引号通信地址)</button>
                        <button onclick={() => handleInsertVar('{{ ._node.id | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block">._node.id (机器标识)</button>
                        <button onclick={() => handleInsertVar('{{ ._node.ssh_host | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block">._node.ssh_host (SSH地址)</button>
                      </div>

                      <div class="border-t border-zinc-100 pt-1.5 dark:border-zinc-800">
                        <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1 mb-1">安全与机密凭据 (SECURITY)</span>
                        <button onclick={() => handleInsertVar('{{ ._sec.uuid | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block text-rose-600 dark:text-rose-400">._sec.uuid (连接凭据 UUID)</button>
                        <button onclick={() => handleInsertVar('{{ ._sec.password | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block text-rose-600 dark:text-rose-400">._sec.password (连接密码)</button>
                      </div>

                      <div class="border-t border-zinc-100 pt-1.5 dark:border-zinc-800">
                        <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1 mb-1">网络子网环境 (NETWORK)</span>
                        <button onclick={() => handleInsertVar('{{ ._net.easytier_cidr | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block text-emerald-600 dark:text-emerald-400">._net.easytier_cidr (组网网段)</button>
                      </div>

                      <div class="border-t border-zinc-100 pt-1.5 dark:border-zinc-800">
                        <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1 mb-1">方案元数据 (PROFILE)</span>
                        <button onclick={() => handleInsertVar('{{ ._profile.token | json }}')} class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[11px] block">._profile.token (订阅安全Token)</button>
                      </div>
                    </div>
                  {/if}

                  <button onclick={saveStudioComp} class="rounded-md bg-blue-600 px-3.5 py-1 text-xs font-semibold text-white hover:bg-blue-500 cursor-pointer">保存母版</button>
                </div>
              </div>

              <div class="flex-1 overflow-hidden p-3">
                <CodeEditor
                  bind:this={editorRef}
                  value={studioTmplCode}
                  dark={ws.isDark}
                  onChange={(newVal: string) => { studioTmplCode = newVal; runStudioPreview(); }}
                />
              </div>

              <div class="border-t border-zinc-200 bg-zinc-50/70 p-4 dark:border-zinc-800 dark:bg-zinc-950/70 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-zinc-700 dark:text-zinc-300">自动侦测参数桩 (Mock Inputs):</span>
                  <span class="text-[10px] text-zinc-400 font-mono">检测到 {studioDetectedVars.length} 个模板插孔</span>
                </div>

                {#if studioDetectedVars.length === 0}
                  <div class="text-[11px] text-zinc-400 italic">当前模板无动态变量，纯静态输出。</div>
                {:else}
                  <div class="grid grid-cols-2 md:grid-cols-4 gap-2.5">
                    {#each studioDetectedVars as v (v)}
                      <label class="block">
                        <span class="text-[10px] font-mono text-zinc-500 block mb-0.5 truncate" title={v}>.{v}</span>
                        <input
                          value={studioTestParams[v] ?? ''}
                          oninput={(e) => handleTestParamInput(v, (e.target as HTMLInputElement).value)}
                          placeholder="填入测试值"
                          class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-900 focus:outline-none focus:ring-1 focus:ring-blue-500"
                        />
                      </label>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>

            <div class="flex w-[45%] flex-col bg-zinc-50/50 dark:bg-zinc-950/80">
              <div class="flex h-12 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
                <span class="font-mono text-xs font-semibold text-zinc-500">RENDERED JSON SANDBOX</span>
                {#if studioPreviewError}
                  <span class="text-[11px] text-rose-500 font-mono font-bold flex items-center gap-1">
                    <Icon name="close" size={12} />
                    <span>语法校验未通过</span>
                  </span>
                {:else}
                  <span class="text-[10px] text-emerald-500 font-semibold flex items-center gap-1">
                    <Icon name="check" size={12} />
                    <span>100% 合法 JSON</span>
                  </span>
                {/if}
              </div>

              {#if studioPreviewError}
                <div class="m-3 p-3 rounded-lg bg-rose-50 text-rose-800 border border-rose-200 dark:bg-rose-950/60 dark:text-rose-200 dark:border-rose-800 text-[11px] font-mono whitespace-pre-wrap overflow-y-auto max-h-48 shadow-inner">
                  {studioPreviewError}
                </div>
              {/if}

              <div class="flex-1 overflow-hidden p-3 pt-0">
                <CodeEditor value={studioPreviewJSON} readonly={true} dark={ws.isDark} />
              </div>
            </div>
          </main>

        <!-- ==================== 工作区 4: 节点管理 (Fleet) ==================== -->
        {:else if ws.currentWorkspace === 'fleet'}
          <div class="flex-1 overflow-y-auto p-8 max-w-6xl mx-auto w-full space-y-6">
            <div class="flex items-center justify-between border-b border-zinc-200 pb-4 dark:border-zinc-800">
              <div>
                <h2 class="text-lg font-bold text-zinc-900 dark:text-zinc-100 flex items-center gap-2">
                  <Icon name="server" color="blue" size={19} />
                  <span>网络节点与实例 (Nodes & Instances)</span>
                </h2>
                <p class="text-xs text-zinc-400 mt-1">管理基础设施中的物理与云端实例。支持原生双栈多通信端点，具备四步安全防炸服推送与热重载确认。</p>
              </div>
              <div class="flex items-center gap-2">
                <button onclick={() => { isDeviceManageMode = !isDeviceManageMode; selectedDeviceIds = []; }} class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 shadow-sm hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer flex items-center gap-1.5">
                  <Icon name="sliders" size={13} />
                  <span>{isDeviceManageMode ? '退出管理' : '批量整理'}</span>
                </button>
                <button disabled={ws.isPingAllLoading} onclick={() => ws.pingAllNodes()} class="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-xs font-semibold text-zinc-700 shadow-sm hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer disabled:opacity-50 flex items-center gap-1.5">
                  <Icon name="refresh" size={13} class={ws.isPingAllLoading ? 'animate-spin' : ''} />
                  <span>{ws.isPingAllLoading ? '并发探测中...' : '一键全网探活'}</span>
                </button>
                <button onclick={() => { editingDevice = null; showDeviceModal = true; }} class="rounded-lg bg-blue-600 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-blue-500 shadow-sm transition cursor-pointer flex items-center gap-1.5">
                  <Icon name="plus" size={14} />
                  <span>接入新节点</span>
                </button>
              </div>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              {#each ws.devices as dev (dev.id)}
                <div class="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-700 transition space-y-4">
                  <div class="flex items-start justify-between">
                    <div class="flex items-start gap-3">
                      {#if isDeviceManageMode}
                        <input type="checkbox" checked={selectedDeviceIds.includes(dev.id)} onchange={(e) => {
                          if ((e.target as HTMLInputElement).checked) selectedDeviceIds = [...selectedDeviceIds, dev.id];
                          else selectedDeviceIds = selectedDeviceIds.filter(id => id !== dev.id);
                        }} class="rounded mt-1" />
                      {/if}
                      <div>
                        <div class="flex items-center gap-2">
                          <h3 class="text-sm font-bold text-zinc-900 dark:text-zinc-100">{dev.name}</h3>
                          <span class="rounded bg-zinc-100 dark:bg-zinc-800 px-1.5 py-0.5 font-mono text-[10px] text-zinc-400">{dev.id}</span>
                        </div>

                        {#if dev.tags && dev.tags.length > 0}
                          <div class="flex flex-wrap gap-1 mt-1.5">
                            {#each dev.tags as tag (tag)}
                              <span class="rounded bg-zinc-100 px-1.5 py-0.2 text-[10px] text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400 font-mono">#{tag}</span>
                            {/each}
                          </div>
                        {/if}
                      </div>
                    </div>

                    <div class="flex items-center gap-2">
                      {#if ws.nodePingStatuses[dev.id]?.loading}
                        <span class="font-mono text-xs text-zinc-400 animate-pulse flex items-center gap-1">
                          <Icon name="refresh" size={12} class="animate-spin" />
                          <span>探活中</span>
                        </span>
                      {:else if ws.nodePingStatuses[dev.id]?.online}
                        {@const ms = ws.nodePingStatuses[dev.id]?.latency ?? 0}
                        <div class="flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-mono text-xs font-semibold {ms < 100 ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400' : ms < 250 ? 'bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400' : 'bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400'}">
                          <span class="h-1.5 w-1.5 rounded-full {ms < 100 ? 'bg-emerald-500 shadow-[0_0_6px_#10b981]' : ms < 250 ? 'bg-amber-500 shadow-[0_0_6px_#f59e0b]' : 'bg-rose-500 shadow-[0_0_6px_#f43f5e]'}"></span>
                          <span>{ms}ms</span>
                        </div>
                      {:else if ws.nodePingStatuses[dev.id]?.online === false}
                        <span title={ws.nodePingStatuses[dev.id]?.error} class="flex items-center gap-1 text-xs font-mono text-rose-500">
                          <span class="h-1.5 w-1.5 rounded-full bg-rose-500"></span>
                          <span>阻断</span>
                        </span>
                      {:else}
                        <span class="font-mono text-[11px] text-zinc-400">未探测</span>
                      {/if}

                      <div class="flex items-center border-l border-zinc-200 pl-2 dark:border-zinc-800 gap-1 text-zinc-400">
                        <button title="重新探活" onclick={() => ws.pingNode(dev.id)} class="p-1 hover:text-blue-500 cursor-pointer">
                          <Icon name="refresh" size={13} />
                        </button>
                        <button title="克隆节点" onclick={() => cloneDevice(dev)} class="p-1 hover:text-blue-500 cursor-pointer">
                          <Icon name="copy" size={13} />
                        </button>
                        <button title="编辑节点元数据" onclick={() => { editingDevice = dev; showDeviceModal = true; }} class="p-1 hover:text-blue-500 cursor-pointer">
                          <Icon name="pencil" size={13} />
                        </button>
                        <button title="移除节点" onclick={() => triggerConfirm('确认移除节点？', `将从系统中移除节点 [${dev.name}]。`, () => ws.batchDeleteDevices([dev.id]))} class="p-1 hover:text-rose-500 cursor-pointer">
                          <Icon name="trash" size={13} />
                        </button>
                      </div>
                    </div>
                  </div>

                  <div class="rounded-xl bg-zinc-50/70 p-3 dark:bg-zinc-950/60 border border-zinc-100 dark:border-zinc-800/80 space-y-2 text-xs">
                    <div class="flex items-start justify-between">
                      <span class="text-zinc-400 font-mono text-[11px]">通信端点:</span>
                      <div class="space-y-0.5 text-right font-mono text-[11px]">
                        {#each dev.endpoints as ep (ep)}
                          <div class="text-zinc-700 dark:text-zinc-300">{ep}</div>
                        {:else}
                          <span class="text-zinc-400">未配置</span>
                        {/each}
                      </div>
                    </div>

                    <div class="flex items-center justify-between border-t border-zinc-200/50 pt-2 dark:border-zinc-800/50">
                      <span class="text-zinc-400 text-[11px]">生效方案:</span>
                      {#if dev.target_profile_id}
                        <button
                          onclick={() => {
                            ws.switchWorkspace('studio');
                            if (dev.target_profile_id) {
                              ws.selectProfile(dev.target_profile_id);
                            }
                          }}
                          class="rounded bg-blue-50 px-2 py-0.5 text-[11px] font-semibold text-blue-600 hover:bg-blue-100 dark:bg-blue-950/40 dark:text-blue-400 cursor-pointer transition flex items-center gap-1"
                        >
                          <Icon name="layers" size={11} />
                          <span>{ws.profiles.find(p => p.id === dev.target_profile_id)?.name || dev.target_profile_id}</span>
                        </button>
                      {:else}
                        <span class="text-zinc-400 text-[11px]">未指派</span>
                      {/if}
                    </div>

                    <div class="flex items-center justify-between border-t border-zinc-200/50 pt-2 dark:border-zinc-800/50 font-mono text-[11px]">
                      <span class="text-zinc-400">SSH 运维目标:</span>
                      <span class="text-zinc-600 dark:text-zinc-400 truncate max-w-55">
                        {dev.ssh_user}@{dev.ssh_host || dev.endpoints[0] || '-'}:{dev.ssh_port}
                      </span>
                    </div>
                  </div>

                  <div class="flex items-center justify-between pt-1">
                    <span class="text-[10px] text-zinc-400 font-mono truncate max-w-50" title={dev.remote_path}>
                      {dev.remote_path}
                    </span>

                    <button
                      disabled={isDeploying || !dev.target_profile_id || !dev.enable_push}
                      onclick={() => deployCurrentDevice(dev.id)}
                      class="rounded-lg bg-blue-600 px-3 py-1 text-xs font-semibold text-white hover:bg-blue-500 disabled:opacity-40 cursor-pointer shadow-sm transition flex items-center gap-1.5"
                    >
                      <Icon name="terminal" size={13} />
                      <span>{isDeploying ? '预检推送中...' : '安全推送到实例'}</span>
                    </button>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <!-- 批量管理浮岛 -->
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
      </div>
    {/key}
  </div>
</div>

<MountDrawer open={showMountDrawer} onClose={async () => { showMountDrawer = false; await ws.loadProfiles(); }} />
<SettingsDrawer open={ws.showSettingsDrawer} onClose={() => (ws.showSettingsDrawer = false)} />
<ComponentModal open={showNewCompModal} onClose={() => { showNewCompModal = false; }} onCreate={handleCreateComponent} />
<ModuleModal open={showModuleModal} editModule={editingModule} onClose={() => { showModuleModal = false; editingModule = null; }} onSave={handleSaveModule} />
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