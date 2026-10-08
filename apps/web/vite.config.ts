import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'

// Ports follow the repo's 49xx scheme (see CLAUDE.md). /api goes to the API
// through this proxy so the session cookie stays same-origin.
export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  // shadcn-svelte's components import from $lib, as in SvelteKit.
  resolve: { alias: { $lib: fileURLToPath(new URL('./src/lib', import.meta.url)) } },
  server: {
    host: '127.0.0.1',
    port: 4930,
    strictPort: true,
    // xfwd passes the address the browser used on, for links the API builds
    // (the Desktop sign-in's approve link).
    proxy: { '/api': { target: 'http://127.0.0.1:4910', changeOrigin: true, xfwd: true } },
  },
})
