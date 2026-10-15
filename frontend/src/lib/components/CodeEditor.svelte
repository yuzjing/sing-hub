<!-- frontend/src/lib/components/CodeEditor.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { EditorView, basicSetup } from 'codemirror';
  import { EditorState, Compartment } from '@codemirror/state';
  import { json } from '@codemirror/lang-json';
  import { oneDark } from '@codemirror/theme-one-dark';

  let {
    value = '',
    readonly = false,
    dark = true,
    onChange
  }: {
    value?: string;
    readonly?: boolean;
    dark?: boolean;
    onChange?: (val: string) => void;
  } = $props();

  let editorElement: HTMLDivElement;
  let view: EditorView | null = null;
  const themeCompartment = new Compartment();

  // 关键：导出给父组件调用的“光标处精确插入”方法
  export function insertAtCursor(snippet: string) {
    if (!view) return;
    const ranges = view.state.selection.ranges;
    const mainRange = ranges[0] ?? { from: view.state.doc.length, to: view.state.doc.length };
    
    view.dispatch({
      changes: { from: mainRange.from, to: mainRange.to, insert: snippet },
      selection: { anchor: mainRange.from + snippet.length },
    });
    view.focus();
  }

  onMount(() => {
    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged && onChange) {
        onChange(update.state.doc.toString());
      }
    });

    const state = EditorState.create({
      doc: value,
      extensions: [
        basicSetup,
        json(),
        themeCompartment.of(dark ? oneDark : []),
        EditorView.editable.of(!readonly),
        updateListener,
        EditorView.theme({
          '&': { height: '100%', fontSize: '13px' },
          '.cm-scroller': { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace', lineHeight: '1.6' },
          '&.cm-focused': { outline: 'none' }
        })
      ],
    });

    view = new EditorView({ state, parent: editorElement });
    return () => view?.destroy();
  });

  $effect(() => {
    if (view && value !== view.state.doc.toString()) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: value },
      });
    }
  });

  $effect(() => {
    if (view) {
      view.dispatch({
        effects: themeCompartment.reconfigure(dark ? oneDark : [])
      });
    }
  });
</script>

<div 
  bind:this={editorElement} 
  class="h-full w-full overflow-hidden rounded font-mono transition-colors"
></div>