import { authenticateRequest } from '$lib/server/auth.js';
import { getConfig } from '$lib/server/config.js';

let workersStarted = false;
let taskModulePromise;

// Ensure configuration is loaded and environment variables (e.g. DATABASE_URL)
// are populated before any background workers connect to Postgres.
getConfig();

export const handle = async ({ event, resolve }) => {
  if (!workersStarted) {
    const tasksModule = await (taskModulePromise ??= import('$lib/server/tasks.js'));
    tasksModule.startTaskWorker();
    tasksModule.scheduleAnalyticsCollector();
    tasksModule.scheduleCrashDetector();
    workersStarted = true;
  }
  event.locals.user = await authenticateRequest(event.request);
  return resolve(event);
};
