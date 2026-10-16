<!-- frontend/src/lib/components/workspaces/StudioWorkspace.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import { api } from '$lib/api';
  import type { Profile, Module, Component } from '$lib/types';
  import CodeEditor from '$lib/components/CodeEditor.svelte';
  import Icon from '$lib/components/Icon.svelte';

  let {
    isProfileManageMode = $bindable(false),
    selectedProfileIds = $bindable<string[]>([]),
    onEditProfile = () => {},
    onCloneProfile = () => {},
    onNewProfile = () => {},
    onOpenMount = () => {},
    triggerConfirm = () => {}
  } = $props<{
    isProfileManageMode?: boolean;
    selectedProfileIds?: string[];
    onEditProfile?: (p: Profile) => void;
    onCloneProfile?: (p: Profile) => void;
    onNewProfile?: () => void;
    onOpenMount?: () => void;
    triggerConfirm?: (title: string, content: string, onConfirm: () => void) => void;
  }>();

  let copiedJSON = $state(false);
  let copiedSub = $state(false);

  // 就地展开的模块 ID 与实参草稿
  let expandedModuleId = $state<string | null>(null);
  let inlineDraftParams = $state<Record<string, string | number | boolean>>({});
  let isInlineDirty = $state(false);

  function safeParse(raw: string | undefined): Record<string, string | number | boolean> {
    if (!raw) return {};
    try {
      return JSON.parse(raw);
    } catch {
      return {};
    }
  }

  // 展开时严格拉取该积木最新已保存的实参作为基准
  function toggleInlineExpand(mod: Module) {
    if (expandedModuleId === mod.id) {
      expandedModuleId = null;
      isInlineDirty = false;
    } else {
      expandedModuleId = mod.id;
      // 保证取的是全局库中的最新实体
      const latestMod = ws.modules.find(m => m.id === mod.id) || mod;
      inlineDraftParams = safeParse(latestMod.params_json);
      isInlineDirty = false;
    }
  }

  function getTemplateVars(tmpl: Component | null): string[] {
    if (!tmpl || !tmpl.payload_tmpl) return [];
    const matches = tmpl.payload_tmpl.matchAll(/\{\{\s*\.([a-zA-Z0-9_.]+)(?:\s*\|[^}]+)?\s*\}\}/g);
    const unique: string[] = [];
    for (const m of matches) {
      if (!unique.includes(m[1])) unique.push(m[1]);
    }
    return unique;
  }

  async function saveInlineParams(mod: Module) {
    const latestMod = ws.modules.find(m => m.id === mod.id) || mod;
    latestMod.params_json = JSON.stringify(inlineDraftParams);
    await api.saveModule(latestMod);
    await ws.loadModules();
    await ws.loadProfiles();
    isInlineDirty = false;
    ws.showToast(`积木 [${mod.name}] 实参已更新并重新编译 ✓`, 'success');
    if (ws.activeProfileId) ws.triggerCompile();
  }

  function cancelInlineParams(mod: Module) {
    const latestMod = ws.modules.find(m => m.id === mod.id) || mod;
    inlineDraftParams = safeParse(latestMod.params_json);
    isInlineDirty = false;
    ws.showToast('已还原为该积木固化实参', 'info');
  }

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
</script>

<div class="flex flex-1 min-h-0 overflow-hidden relative">
  <!-- 左栏：方案配方列表 -->
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
          onclick={onNewProfile}
          class="flex h-6 w-6 items-center justify-center rounded text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/50 cursor-pointer"
        >
          <Icon name="plus" size={15} />
        </button>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto p-2 space-y-1">
      {#each ws.profiles as p (p.id)}
        <div class="group flex items-center justify-between rounded-lg px-2.5 py-2 text-xs transition {ws.activeProfileId === p.id && !isProfileManageMode ? 'bg-white shadow-xs ring-1 ring-zinc-300 text-zinc-900 font-semibold dark:bg-zinc-800/90 dark:ring-zinc-700 dark:text-white' : 'text-zinc-600 hover:bg-zinc-200/60 dark:text-zinc-400 dark:hover:bg-zinc-900'}">
          <div class="flex items-center gap-2 truncate flex-1">
            {#if isProfileManageMode}
              <input
                type="checkbox"
                checked={selectedProfileIds.includes(p.id)}
                onchange={(e) => {
                  if ((e.target as HTMLInputElement).checked) selectedProfileIds = [...selectedProfileIds, p.id];
                  else selectedProfileIds = selectedProfileIds.filter((id: string) => id !== p.id);
                }}
                class="rounded cursor-pointer"
              />
            {/if}
            <button onclick={() => ws.selectProfile(p.id)} class="truncate text-left flex-1 cursor-pointer">
              {p.name}
            </button>
          </div>

          <div class="flex items-center gap-1">
            {#if !isProfileManageMode}
              {#if p.capabilities.enable_subscribe}
                <button onclick={() => copySubURL(p.token)} title="复制订阅链接" class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                  <Icon name="link" size={13} />
                </button>
              {/if}
            {:else}
              <button title="克隆方案" onclick={() => onCloneProfile(p)} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
                <Icon name="copy" size={13} />
              </button>
              <button title="编辑元属性" onclick={() => onEditProfile(p)} class="text-zinc-400 hover:text-blue-500 cursor-pointer p-0.5">
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

  <!-- 中间流水线 -->
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
        <div class="flex items-center gap-2">
          <button
            onclick={onOpenMount}
            class="rounded-md border border-zinc-300 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700 shadow-2xs hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer transition flex items-center gap-1.5"
          >
            <Icon name="plus" size={13} />
            <span>挑选装配</span>
          </button>
          <button
            onclick={() => ws.triggerCompile()}
            class="rounded-md bg-blue-600 px-3 py-1 text-xs font-semibold text-white shadow-2xs hover:bg-blue-500 cursor-pointer transition flex items-center gap-1.5"
          >
            <Icon name="check" size={13} />
            <span>{ws.isCompiling ? '编译中...' : '编译保存'}</span>
          </button>
        </div>
      {/if}
    </div>

    <div class="flex-1 overflow-y-auto p-5 space-y-3">
      {#if ws.resolvedModuleBindings.length === 0}
        <div class="flex h-80 flex-col items-center justify-center rounded-xl border border-dashed border-zinc-300 p-8 text-center dark:border-zinc-800">
          <span class="text-xs text-zinc-400">当前方案暂未装配成品积木，点击右上角「挑选装配」添加</span>
        </div>
      {:else}
        {#each ws.resolvedModuleBindings as item, idx (item.binding.id ?? idx)}
          {@const latestMod = ws.modules.find(m => m.id === item.module.id) || item.module}
          {@const currentParams = safeParse(latestMod.params_json)}
          {@const tmplVars = getTemplateVars(item.template)}
          {@const isExpanded = expandedModuleId === item.module.id}

          <div class="rounded-xl border border-zinc-200 bg-zinc-50/70 p-4 shadow-xs hover:border-zinc-300 dark:border-zinc-800/90 dark:bg-zinc-900/60 dark:hover:border-zinc-700 transition space-y-2.5">
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
                  onclick={() => toggleInlineExpand(item.module)}
                  class="rounded border border-zinc-200 bg-white px-2 py-0.5 text-xs text-zinc-600 hover:border-zinc-300 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-300 cursor-pointer"
                >
                  {isExpanded ? '收起配置' : '调整实参'}
                </button>
                <button disabled={idx === 0} onclick={() => ws.moveModuleBinding(idx, 'up')} class="rounded px-2 py-0.5 text-xs text-zinc-500 hover:bg-zinc-200 disabled:opacity-30 dark:hover:bg-zinc-800 cursor-pointer">↑</button>
                <button disabled={idx === ws.resolvedModuleBindings.length - 1} onclick={() => ws.moveModuleBinding(idx, 'down')} class="rounded px-2 py-0.5 text-xs text-zinc-500 hover:bg-zinc-200 disabled:opacity-30 dark:hover:bg-zinc-800 cursor-pointer">↓</button>
                <button onclick={() => ws.detachModule(item.binding.id ?? item.module.id)} class="rounded p-1 text-zinc-400 hover:text-rose-500 cursor-pointer">
                  <Icon name="close" size={13} />
                </button>
              </div>
            </div>

            <!-- 就地展开且完整预填已有实参 -->
            {#if isExpanded}
              <div class="pt-3 border-t border-zinc-200/70 dark:border-zinc-800/80 space-y-3 animate-in fade-in duration-100">
                <div class="flex items-center justify-between">
                  <span class="text-[11px] font-bold text-zinc-700 dark:text-zinc-300">
                    实参赋值 (保存后右侧全局配置实时联动编译):
                  </span>
                  <div class="flex items-center gap-2">
                    {#if isInlineDirty}
                      <button
                        onclick={() => cancelInlineParams(item.module)}
                        class="text-[11px] text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200 cursor-pointer"
                      >
                        放弃
                      </button>
                    {/if}
                    <button
                      onclick={() => saveInlineParams(item.module)}
                      class="rounded bg-blue-600 px-2.5 py-0.5 text-[11px] font-semibold text-white hover:bg-blue-500 cursor-pointer shadow-2xs"
                    >
                      应用实参
                    </button>
                  </div>
                </div>

                {#if tmplVars.length === 0}
                  <div class="text-[11px] text-zinc-400 italic">该母版无动态插孔。</div>
                {:else}
                  <div class="grid grid-cols-2 md:grid-cols-3 gap-2.5">
                    {#each tmplVars as v (v)}
                      <label class="block">
                        <span class="text-[10px] font-mono text-zinc-500 block mb-0.5 truncate" title={v}>.{v}</span>
                        <input
                          value={inlineDraftParams[v] ?? currentParams[v] ?? ''}
                          oninput={(e) => {
                            const val = (e.target as HTMLInputElement).value;
                            let parsed: string | number | boolean = val;
                            if (val.trim() !== '' && !isNaN(Number(val))) parsed = Number(val);
                            else if (val.toLowerCase() === 'true') parsed = true;
                            else if (val.toLowerCase() === 'false') parsed = false;
                            inlineDraftParams[v] = parsed;
                            isInlineDirty = true;
                          }}
                          placeholder={v.startsWith('_') ? '默认上下文' : '实参值'}
                          class="w-full rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono dark:border-zinc-700 dark:bg-zinc-950 focus:outline-none focus:ring-1 focus:ring-blue-500"
                        />
                      </label>
                    {/each}
                  </div>
                {/if}
              </div>
            {:else}
              <!-- 收起状态直接展示当前已固化的实参 -->
              <div class="pt-2 border-t border-zinc-200/50 dark:border-zinc-800/50 flex flex-wrap gap-2 text-[11px] font-mono">
                {#each Object.entries(currentParams) as [k, val] (k)}
                  <span class="rounded bg-white dark:bg-zinc-950 px-2 py-0.5 border border-zinc-200 dark:border-zinc-800 text-zinc-600 dark:text-zinc-400">
                    .{k}: <strong class="text-zinc-800 dark:text-zinc-200">{String(val)}</strong>
                  </span>
                {:else}
                  <span class="text-zinc-400 text-[10px] italic">该成品使用纯静态或环境默认值。</span>
                {/each}
              </div>
            {/if}
          </div>
        {/each}
      {/if}
    </div>

    <!-- 底部状态条 -->
    <div class="h-7 border-t border-zinc-200 bg-zinc-50 px-3 flex items-center justify-between text-[11px] font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
      <div class="flex items-center gap-3">
        <span class="flex items-center gap-1.5">
          <span class="h-1.5 w-1.5 rounded-full {ws.compileError ? 'bg-rose-500' : 'bg-emerald-500'}"></span>
          <span>{ws.compileError ? '装配有误' : '流水线就绪'}</span>
        </span>
        {#if ws.activeCommitHash}
          <span>git:{ws.activeCommitHash.slice(0, 7)}</span>
        {/if}
      </div>
      <div class="flex items-center gap-3">
        <span>挂载积木: {ws.resolvedModuleBindings.length} 项</span>
      </div>
    </div>
  </main>

  <!-- 右侧编译镜像 -->
  <section class="flex w-[42%] min-w-95 max-w-155 shrink-0 flex-col bg-zinc-50 dark:bg-zinc-950/90">
    <div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800">
      <div class="flex items-center gap-2">
        <span class="font-mono text-xs font-bold text-zinc-500 dark:text-zinc-400">CONFIG MIRROR</span>
        {#if ws.activeCommitHash}
          <button onclick={() => ws.openTimeline()} class="flex items-center gap-1 rounded bg-zinc-200/80 px-2 py-0.5 font-mono text-[10px] font-semibold text-zinc-700 hover:bg-zinc-300 dark:bg-zinc-800 dark:text-zinc-300 cursor-pointer">
            <span>git: {ws.activeCommitHash.slice(0, 7)}</span>
          </button>
        {/if}
      </div>

      <div class="flex items-center gap-2">
        <button onclick={copyCompiledJSON} class="rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono font-medium text-zinc-700 shadow-2xs hover:border-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1">
          <Icon name="copy" size={12} />
          <span>{copiedJSON ? '已复制' : '复制 JSON'}</span>
        </button>
        {#if ws.activeProfile?.capabilities.enable_subscribe}
          <button onclick={() => copySubURL(ws.activeProfile?.token ?? '')} class="rounded border border-zinc-300 bg-white px-2 py-1 text-xs font-mono font-medium text-zinc-700 shadow-2xs hover:border-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 cursor-pointer flex items-center gap-1">
            <Icon name="link" size={12} />
            <span>{copiedSub ? '已复制' : '复制订阅'}</span>
          </button>
        {/if}
      </div>
    </div>

    {#if ws.compileError}
      <div class="m-3 p-3 rounded-lg bg-rose-50 text-rose-800 border border-rose-200 dark:bg-rose-950/60 dark:text-rose-200 dark:border-rose-800 text-[11px] font-mono whitespace-pre-wrap overflow-y-auto max-h-48 shadow-inner">
        {ws.compileError}
      </div>
    {/if}

    <div class="flex-1 overflow-hidden p-3 pt-0">
      <div class="h-full rounded-xl border border-zinc-200 bg-white p-1 shadow-2xs dark:border-zinc-800/80 dark:bg-zinc-950">
        <CodeEditor value={ws.compiledJSON} readonly={true} dark={ws.isDark} />
      </div>
    </div>

    <div class="h-7 border-t border-zinc-200 bg-zinc-50 px-3 flex items-center justify-between text-[11px] font-mono text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900/80">
      <span>{ws.compileError ? '语法校验未通过' : '100% 编译通过'}</span>
      <span>大小: {(new Blob([ws.compiledJSON]).size / 1024).toFixed(1)} KB</span>
    </div>
  </section>
</div>