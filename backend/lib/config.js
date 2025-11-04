import { config as loadEnv } from 'dotenv';
import { join } from 'node:path';

const envLoaded = loadEnv();

const env = process.env;

const defaultRoot = env.NODE_ENV === 'production'
  ? '/var/lib/bakery'
  : join(process.cwd(), '.data');

const rootDir = env.BAKERY_ROOT || defaultRoot;

export function getConfig() {
  return {
    envLoaded,
    environment: env.NODE_ENV || 'development',
    host: env.BAKERY_HOST || '0.0.0.0',
    port: Number(env.BAKERY_PORT || 4100),
    baseUrl: env.BAKERY_BASE_URL || 'http://localhost:5173',
    apiUrl: env.BAKERY_API_URL || 'http://localhost:4100',
    databaseUrl:
      env.DATABASE_URL ||
      'postgres://bakery:bakery@localhost:5432/bakery',
    sessionSecret:
      env.SESSION_SECRET ||
      'dev-insecure-session-secret-change-me',
    encryptionKey:
      env.ENCRYPTION_KEY ||
      '0123456789abcdef0123456789abcdef',
    dataDir: env.BAKERY_DATA_DIR || join(rootDir, 'data'),
    logsDir: env.BAKERY_LOGS_DIR || join(rootDir, 'logs'),
    buildsDir: env.BAKERY_BUILDS_DIR || join(rootDir, 'builds'),
    releasesToKeep: Number(env.BAKERY_RELEASES_KEEP || 5),
    githubClientId: env.GITHUB_CLIENT_ID || '',
    githubClientSecret: env.GITHUB_CLIENT_SECRET || '',
    githubAppWebhookSecret: env.GITHUB_APP_WEBHOOK_SECRET || '',
    systemdServicesDir: env.BAKERY_SYSTEMD_DIR || '/etc/systemd/system',
    nginxSitesDir: env.BAKERY_NGINX_SITES_DIR || '/etc/nginx/conf.d',
    nginxTemplateDir:
      env.BAKERY_NGINX_TEMPLATE_DIR ||
      join(process.cwd(), 'infrastructure', 'nginx', 'templates'),
    certbotEmail: env.CERTBOT_EMAIL || '',
    publicIp: env.BAKERY_PUBLIC_IP || '',
    allowedOrigins: (env.BAKERY_ALLOWED_ORIGINS || '')
      .split(',')
      .map((o) => o.trim())
      .filter(Boolean),
    blueGreenBasePort: Number(env.BAKERY_BASE_PORT || 5200),
    logRetentionDays: Number(env.BAKERY_LOG_RETENTION_DAYS || 30)
  };
}
