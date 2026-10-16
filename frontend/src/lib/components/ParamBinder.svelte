<!-- frontend/src/lib/components/ParamBinder.svelte -->
<script lang="ts">
  import { ws } from '$lib/workspace.svelte';
  import type { Device } from '$lib/types';
  import Icon from '$lib/components/Icon.svelte';

  let {
    templateVars = [],
    params = $bindable<Record<string, string | number | boolean>>({}),
    onChange = () => {}
  } = $props<{
    templateVars: string[];
    params: Record<string, string | number | boolean>;
    onChange?: () => void;
  }>();

  let searchFilter = $state('');
  let activeMenuVar = $state<string | null>(null);

  let filteredVars = $derived.by(() => {
    if (!searchFilter.trim()) return templateVars;
    return templateVars.filter(v => v.toLowerCase().includes(searchFilter.toLowerCase()));
  });

  function isIPv4(val: string) {
    return val.split('.').length === 4 && !val.includes(':');
  }

  function isIPv6(val: string) {
    return val.includes(':');
  }

  function applyRef(varKey: string, refExpr: string) {
    params[varKey] = refExpr;
    activeMenuVar = null;
    onChange();
  }

  function handleInputChange(varKey: string, rawVal: string) {
    let parsed: string | number | boolean = rawVal;
    if (rawVal.trim() !== '' && !isNaN(Number(rawVal))) {
      parsed = Number(rawVal);
    } else if (rawVal.toLowerCase() === 'true') {
      parsed = true;
    } else if (rawVal.toLowerCase() === 'false') {
      parsed = false;
    }
    params[varKey] = parsed;
    onChange();
  }
</script>

<div class="space-y-3">
  <!-- 数量较多时展示轻量快速检索栏 -->
  {#if templateVars.length > 4}
    <div class="flex items-center justify-between gap-2 border-b border-zinc-100 pb-2 dark:border-zinc-800">
      <span class="text-[10px] text-zinc-400 font-mono">共 {templateVars.length} 个参数插孔</span>
      <input
        bind:value={searchFilter}
        placeholder="搜索插孔变量名..."
        class="rounded-md border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-2 py-0.5 text-[11px] font-mono focus:outline-none focus:border-blue-500 w-44"
      />
    </div>
  {/if}

  {#if filteredVars.length === 0}
    <div class="text-xs text-zinc-400 italic py-4">无匹配的参数插孔。</div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3 max-h-80 overflow-y-auto pr-1">
      {#each filteredVars as v (v)}
        {@const curVal = String(params[v] ?? '')}
        {@const isRef = curVal.startsWith('@node:') || curVal.startsWith('@net:') || curVal.startsWith('@sec:')}

        <div class="relative rounded-xl border border-zinc-200/80 bg-white p-2.5 shadow-2xs dark:border-zinc-800 dark:bg-zinc-950 space-y-1.5">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-mono font-bold text-zinc-700 dark:text-zinc-300 truncate" title={v}>
              .{v}
            </span>

            <!-- 优雅的层级树菜单触发器 -->
            <div class="relative">
              <button
                onclick={() => activeMenuVar = activeMenuVar === v ? null : v}
                class="flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium {isRef ? 'bg-blue-50 text-blue-600 dark:bg-blue-950/60 dark:text-blue-400 font-bold' : 'bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400 hover:bg-zinc-200 dark:hover:bg-zinc-700'} transition cursor-pointer"
              >
                <span>{isRef ? '🔗 已引用' : '⚡ 引用系统/节点'}</span>
                <span class="text-[8px] opacity-70">▾</span>
              </button>

              <!-- 层级树浮动菜单 -->
              {#if activeMenuVar === v}
                <!-- 遮罩用于点击外部关闭 -->
                <button
                  type="button"
                  tabindex="-1"
                  aria-label="关闭引用菜单"
                  class="fixed inset-0 z-40 bg-transparent cursor-default border-none p-0 w-full h-full"
                  onclick={() => activeMenuVar = null}
                ></button>

                <div class="absolute right-0 top-6 z-50 w-72 rounded-xl border border-zinc-200 bg-white p-2 shadow-2xl dark:border-zinc-700 dark:bg-zinc-900 text-xs space-y-2.5 animate-in fade-in duration-100 max-h-80 overflow-y-auto">
                  
                  <!-- 1. 网络节点集群 (分 IPV4 / IPV6 / 域名多端点) -->
                  <div>
                    <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1.5 mb-1 flex items-center gap-1">
                      <Icon name="server" size={11} color="blue" />
                      <span>网络节点端点与域名</span>
                    </span>

                    {#if ws.devices.length === 0}
                      <span class="text-[10px] text-zinc-400 italic px-2 block">暂无接入节点</span>
                    {:else}
                      <div class="space-y-1">
                        {#each ws.devices as dev (dev.id)}
                          <div class="p-1.5 rounded-lg hover:bg-zinc-50 dark:hover:bg-zinc-800/60 border border-transparent hover:border-zinc-100 dark:hover:border-zinc-800">
                            <span class="text-[11px] font-bold text-zinc-800 dark:text-zinc-200 block truncate">
                              {dev.name}
                            </span>
                            
                            <div class="grid grid-cols-2 gap-1 mt-1 font-mono text-[10px]">
                              <!-- 智能首选端点 -->
                              <button
                                onclick={() => applyRef(v, `@node:${dev.id}.endpoint`)}
                                class="text-left px-1.5 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 text-blue-600 dark:text-blue-400 hover:bg-blue-100 truncate cursor-pointer"
                                title="引用主端点 ({dev.endpoints[0] || '默认'})"
                              >
                                主端点
                              </button>

                              <!-- 智能 IPv4 -->
                              {#if dev.endpoints.some(isIPv4)}
                                <button
                                  onclick={() => applyRef(v, `@node:${dev.id}.ipv4`)}
                                  class="text-left px-1.5 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-100 truncate cursor-pointer"
                                  title="引用节点 IPv4 地址"
                                >
                                  IPv4 端点
                                </button>
                              {/if}

                              <!-- 智能 IPv6 -->
                              {#if dev.endpoints.some(isIPv6)}
                                <button
                                  onclick={() => applyRef(v, `@node:${dev.id}.ipv6`)}
                                  class="text-left px-1.5 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 text-purple-600 dark:text-purple-400 hover:bg-purple-100 truncate cursor-pointer"
                                  title="引用节点 IPv6 地址"
                                >
                                  IPv6 端点
                                </button>
                              {/if}

                              <!-- 智能域名 -->
                              {#if dev.endpoints.some(ep => !isIPv4(ep) && !isIPv6(ep))}
                                <button
                                  onclick={() => applyRef(v, `@node:${dev.id}.domain`)}
                                  class="text-left px-1.5 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 text-amber-600 dark:text-amber-400 hover:bg-amber-100 truncate cursor-pointer"
                                  title="引用节点域名"
                                >
                                  域名端点
                                </button>
                              {/if}
                            </div>
                          </div>
                        {/each}
                      </div>
                    {/if}
                  </div>

                  <!-- 2. 全局环境与网段 -->
                  <div class="border-t border-zinc-100 pt-1.5 dark:border-zinc-800">
                    <span class="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block px-1.5 mb-1">组网与安全</span>
                    <button
                      onclick={() => applyRef(v, '@net:easytier_cidr')}
                      class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[10px] text-emerald-600 block cursor-pointer"
                    >
                      @net:easytier_cidr (组网网段)
                    </button>
                    <button
                      onclick={() => applyRef(v, '@sec:token')}
                      class="w-full text-left px-2 py-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 font-mono text-[10px] text-rose-600 block cursor-pointer"
                    >
                      @sec:token (方案安全凭据)
                    </button>
                  </div>

                  <!-- 3. 清除引用，切回手动 -->
                  {#if isRef}
                    <div class="border-t border-zinc-100 pt-1 dark:border-zinc-800">
                      <button
                        onclick={() => { params[v] = ''; activeMenuVar = null; onChange(); }}
                        class="w-full text-left px-2 py-0.5 text-zinc-400 hover:text-rose-500 text-[10px] block cursor-pointer"
                      >
                        ✕ 解除引用并清空
                      </button>
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          </div>

          <!-- 输入框区 -->
          <div class="relative">
            <input
              value={curVal}
              oninput={(e) => handleInputChange(v, (e.target as HTMLInputElement).value)}
              placeholder={v.startsWith('_') ? '留空使用默认上下文' : '填入具体实参或点击右上角引用'}
              class="w-full rounded-lg border {isRef ? 'border-blue-400/80 bg-blue-50/20 text-blue-700 dark:border-blue-800 dark:text-blue-300' : 'border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-950 text-zinc-900 dark:text-zinc-100'} px-2.5 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>