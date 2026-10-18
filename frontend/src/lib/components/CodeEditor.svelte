<!-- frontend/src/lib/components/CodeEditor.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { EditorView, basicSetup } from 'codemirror';
  import { EditorState, Compartment } from '@codemirror/state';
  import { json } from '@codemirror/lang-json';
  import { oneDark } from '@codemirror/theme-one-dark';

  interface Props {
    value?: string;
    readonly?: boolean;
    dark?: boolean;
    onChange?: (val: string) => void;
  }

  let {
    value = '',
    readonly = false,
    dark = true,
    onChange = () => {}
  }: Props = $props();

  let editorElement: HTMLDivElement;
  let view: EditorView | null = null;
  const themeCompartment = new Compartment();
  const readonlyCompartment = new Compartment();

  export function insertAtCursor(snippet: string) {
    if (!view) return;
    const { from, to } = view.state.selection.main;
    view.dispatch({
      changes: { from, to, insert: snippet },
      selection: { anchor: from + snippet.length }
    });
    view.focus();
  }

  onMount(() => {
    const extensions = [
      basicSetup,
      json(),
      EditorView.lineWrapping,
      readonlyCompartment.of(EditorState.readOnly.of(readonly)),
      themeCompartment.of(dark ? oneDark : []),
      EditorView.updateListener.of((update: { docChanged: boolean; state: { doc: { toString(): string } } }) => {
        if (update.docChanged && !readonly) {
          onChange(update.state.doc.toString());
        }
      })
    ];

    view = new EditorView({
      doc: value || '',
      extensions,
      parent: editorElement
    });

    return () => {
      view?.destroy();
      view = null;
    };
  });

  // 关键：外部数据到达时实时刷新编辑器，彻底解决右侧空白
  $effect(() => {
    const targetVal = value || '';
    if (view && view.state.doc.toString() !== targetVal) {
      view.dispatch({
        changes: {
          from: 0,
          to: view.state.doc.length,
          insert: targetVal
        }
      });
    }
  });

  // 关键：动态响应浅色/深色主题切换，彻底消除只有编辑器是黑底的问题
  $effect(() => {
    if (view) {
      view.dispatch({
        effects: [
          themeCompartment.reconfigure(dark ? oneDark : []),
          readonlyCompartment.reconfigure(EditorState.readOnly.of(readonly))
        ]
      });
    }
  });
</script>

<div class="h-full w-full overflow-hidden font-mono text-xs" bind:this={editorElement}></div>

<style>
  :global(.cm-editor) {
    height: 100% !important;
  }
  :global(.cm-scroller) {
    overflow: auto !important;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace !important;
  }
</style>