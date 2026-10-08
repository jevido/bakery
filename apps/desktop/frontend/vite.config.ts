import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'
import wails from '@wailsio/runtime/plugins/vite'
import { defineConfig } from 'vite'

// wails3 dev runs this on 4990 (CLAUDE.md's ports; WAILS_VITE_PORT from the
// Taskfile). Its plugin serves the generated bindings in dev.
export default defineConfig({
  plugins: [tailwindcss(), svelte(), wails('./bindings')],
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 4990,
    strictPort: true,
  },
})
