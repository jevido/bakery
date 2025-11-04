import { mkdir } from 'node:fs/promises';
import { router } from './routes/index.js';
import { getConfig } from './lib/config.js';
import { log } from './lib/logger.js';
import { startTaskWorker, scheduleAnalyticsCollector, scheduleCrashDetector } from './lib/tasks.js';

let kitHandler = null;

async function loadKitHandler() {
  if (kitHandler) return kitHandler;
  try {
    const module = await import('../app/build/index.js');
    kitHandler = module.handler;
    return kitHandler;
  } catch (error) {
    await log('warn', 'Unable to load SvelteKit handler', { error: error.message });
    return null;
  }
}

async function bootstrap() {
  const config = getConfig();
  await mkdir(config.dataDir, { recursive: true });
  await mkdir(config.logsDir, { recursive: true });
  await mkdir(config.buildsDir, { recursive: true });

  startTaskWorker();
  scheduleAnalyticsCollector();
  scheduleCrashDetector();

  await loadKitHandler();

  const server = Bun.serve({
    port: config.port,
    hostname: config.host,
    async fetch(request) {
      const url = new URL(request.url);
      if (url.pathname.startsWith('/api')) {
        return router.handle(request);
      }
      const handler = await loadKitHandler();
      if (handler) {
        return handler(request);
      }
      return router.handle(request);
    },
    websocket: {
      open(ws) {
        ws.send(JSON.stringify({ ok: true }));
      }
    }
  });

  await log('info', 'Bakery backend started', {
    port: config.port,
    environment: config.environment
  });
  return server;
}

bootstrap().catch((error) => {
  console.error(error);
  process.exit(1);
});
