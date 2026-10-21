// frontend/svelte.config.js
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    // 使用静态适配器，并将 fallback 设为 index.html
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html', // 关键！开启单页面应用模式，确保生成 index.html
      precompress: false,
      strict: false
    })
  }
};

export default config;