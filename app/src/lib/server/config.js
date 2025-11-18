import { join } from 'node:path';
import { existsSync } from 'node:fs';

let env = process.env;

try {
	const dynamic = await import('$env/dynamic/private');
	if (dynamic?.env) {
		env = dynamic.env;
	}
} catch (error) {
	// When running outside the SvelteKit runtime (e.g. standalone Bun scripts)
	// the $env module is unavailable. Falling back to process.env keeps those
	// entry points working without additional configuration.
}

const defaultRoot =
	env.NODE_ENV === 'production' ? '/var/lib/bakery' : join(process.cwd(), '.data');

function parseBoolean(value, fallback) {
	if (value == null) return fallback;
	const normalized = String(value).trim().toLowerCase();
	if (['1', 'true', 'yes', 'on'].includes(normalized)) return true;
	if (['0', 'false', 'no', 'off'].includes(normalized)) return false;
	return fallback;
}

const candidateRoots = env.BAKERY_PROJECT_ROOT
	? [env.BAKERY_PROJECT_ROOT]
	: [process.cwd(), join(process.cwd(), '..')];
const projectRoot =
	candidateRoots.find((root) =>
		existsSync(join(root, 'infrastructure', 'nginx', 'templates', 'app.conf'))
	) || process.cwd();
const rootDir = env.BAKERY_ROOT || defaultRoot;
const nodeRootDir = env.BAKERY_NODE_ROOT || '/var/lib/bakery-node';
const nodeLogsDir = env.BAKERY_NODE_LOGS_DIR || '/var/log/bakery-node';

export function getConfig() {
	const localMode = parseBoolean(env.BAKERY_LOCAL_MODE, env.NODE_ENV !== 'production');
	const systemdDirDefault = localMode ? join(rootDir, 'systemd') : '/etc/systemd/system';
	const nginxSitesDefault = localMode
		? join(rootDir, 'nginx', 'sites-enabled')
		: '/etc/nginx/conf.d';

	const config = {
		environment: env.NODE_ENV || 'development',
		localMode,
		host: env.BAKERY_HOST || '0.0.0.0',
		port: Number(env.BAKERY_PORT || 4100),
		baseUrl: env.BAKERY_BASE_URL || 'http://localhost:5173',
		databaseUrl: env.DATABASE_URL || 'postgres://bakery:bakery@localhost:5432/bakery',
		sessionSecret: env.SESSION_SECRET || 'dev-insecure-session-secret-change-me',
		encryptionKey: env.ENCRYPTION_KEY || '0123456789abcdef0123456789abcdef',
		dataDir: env.BAKERY_DATA_DIR || join(rootDir, 'data'),
		logsDir: env.BAKERY_LOGS_DIR || join(rootDir, 'logs'),
		buildsDir: env.BAKERY_BUILDS_DIR || join(rootDir, 'builds'),
		releasesToKeep: Number(env.BAKERY_RELEASES_KEEP || 5),
		githubClientId: env.GITHUB_CLIENT_ID || '',
		githubClientSecret: env.GITHUB_CLIENT_SECRET || '',
		githubAppWebhookSecret: env.GITHUB_APP_WEBHOOK_SECRET || '',
		selfUpdateRepository: env.BAKERY_SELF_REPO || 'jevido/bakery',
		selfUpdateBranch: env.BAKERY_SELF_BRANCH || 'main',
		systemdServicesDir: env.BAKERY_SYSTEMD_DIR || systemdDirDefault,
		nginxSitesDir: env.BAKERY_NGINX_SITES_DIR || nginxSitesDefault,
		nginxTemplateDir:
			env.BAKERY_NGINX_TEMPLATE_DIR || join(projectRoot, 'infrastructure', 'nginx', 'templates'),
		nginxExecutable: env.BAKERY_NGINX_EXECUTABLE || env.NGINX_EXECUTABLE || 'nginx',
		certbotEmail: env.CERTBOT_EMAIL || '',
		publicIp: env.BAKERY_PUBLIC_IP || '',
		allowedOrigins: (env.BAKERY_ALLOWED_ORIGINS || '')
			.split(',')
			.map((o) => o.trim())
			.filter(Boolean),
		blueGreenBasePort: Number(env.BAKERY_BASE_PORT || 5200),
		logRetentionDays: Number(env.BAKERY_LOG_RETENTION_DAYS || 30),
		nodeRootDir,
		nodeDataDir: env.BAKERY_NODE_DATA_DIR || join(nodeRootDir, 'data'),
		nodeBuildsDir: env.BAKERY_NODE_BUILDS_DIR || join(nodeRootDir, 'builds'),
		nodeLogsDir,
		nodeSystemdDir: env.BAKERY_NODE_SYSTEMD_DIR || '/etc/systemd/system',
		nodeNginxSitesDir: env.BAKERY_NODE_NGINX_SITES_DIR || '/etc/nginx/conf.d'
	};

	if (!process.env.DATABASE_URL && config.databaseUrl) {
		process.env.DATABASE_URL = config.databaseUrl;
	}

	return config;
}
