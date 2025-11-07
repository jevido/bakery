import { readFile, writeFile, mkdir, access } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';
import { certificateExists, requestCertificate } from './certbot.js';

async function renderTemplate(templateName, variables) {
	const config = getConfig();
	const templatePath = join(config.nginxTemplateDir, templateName);
	const content = await readFile(templatePath, 'utf8');
	return content.replace(/\{\{(.*?)\}\}/g, (_, key) => {
		const trimmed = key.trim();
		if (!(trimmed in variables)) {
			throw new Error(`Missing template variable ${trimmed}`);
		}
		return variables[trimmed];
	});
}

async function fileExists(path) {
	try {
		await access(path);
		return true;
	} catch {
		return false;
	}
}

async function ensureCertbotDefaults() {
	const required = [
		'/etc/letsencrypt/options-ssl-nginx.conf',
		'/etc/letsencrypt/ssl-dhparams.pem'
	];
	const missing = [];
	for (const path of required) {
		if (!(await fileExists(path))) {
			missing.push(path);
		}
	}
	if (!missing.length) return;

	const script = `import os, shutil
try:
    from certbot_nginx._internal import tls_configs
except Exception:
    raise SystemExit(0)
base = os.path.dirname(tls_configs.__file__)
targets = {
    'options-ssl-nginx.conf': os.path.join('/etc/letsencrypt', 'options-ssl-nginx.conf'),
    'ssl-dhparams.pem': os.path.join('/etc/letsencrypt', 'ssl-dhparams.pem')
}
os.makedirs('/etc/letsencrypt', exist_ok=True)
for name, destination in targets.items():
    source = os.path.join(base, name)
    if os.path.exists(source) and not os.path.exists(destination):
        shutil.copyfile(source, destination)
`;
	const process = spawn(['python3', '-c', script], {
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const stderr = await new Response(process.stderr).text();
	if (process.exitCode !== 0) {
		await log('warn', 'Failed to copy certbot TLS defaults', { stderr });
	}
}

export async function writeDeploymentConfig({ deployment, domains, port, slot, tlsEnabled }) {
	const config = getConfig();
	const domainList = domains.map((d) => d.hostname).join(' ');
	const primaryDomain = domains[0] ? domains[0].hostname : `${deployment.id}.local`;
	const httpsDomains = domains.map((d) => `server_name ${d.hostname};`).join('\n  ');
	const enableTls = typeof tlsEnabled === 'boolean' ? tlsEnabled : await certificateExists(primaryDomain);
	const httpRedirects = enableTls
		? domains
				.map(
					(d) => `
server {
  listen 80;
  server_name ${d.hostname};
  return 301 https://${d.hostname}$request_uri;
}
`
				)
				.join('\n')
		: '';

	const upstreamName = `bakery_${deployment.id}_${slot}`;
	let listenDirective = 'listen 80;';
	let http2Directive = '# http/1.1 only';
	let sslDirectives = '    # TLS disabled until a certificate is available\n';

	if (enableTls) {
		await ensureCertbotDefaults();
		listenDirective = 'listen 443 ssl;';
		http2Directive = 'http2 on;';
		sslDirectives = [
			`    ssl_certificate /etc/letsencrypt/live/${primaryDomain}/fullchain.pem;`,
			`    ssl_certificate_key /etc/letsencrypt/live/${primaryDomain}/privkey.pem;`
		];
		if (await fileExists('/etc/letsencrypt/options-ssl-nginx.conf')) {
			sslDirectives.push('    include /etc/letsencrypt/options-ssl-nginx.conf;');
		}
		if (await fileExists('/etc/letsencrypt/ssl-dhparams.pem')) {
			sslDirectives.push('    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;');
		}
		sslDirectives = sslDirectives.join('\n');
	}

	const nginxBody = await renderTemplate('app.conf', {
		UPSTREAM_NAME: upstreamName,
		PORT: port,
		HTTPS_DOMAINS: httpsDomains,
		HTTP_REDIRECT_BLOCKS: httpRedirects,
		LISTEN_DIRECTIVE: listenDirective,
		HTTP2_DIRECTIVE: http2Directive,
		SSL_DIRECTIVES: sslDirectives,
		ACCESS_LOG: join(config.logsDir, `${deployment.id}-${slot}-access.log`),
		ERROR_LOG: join(config.logsDir, `${deployment.id}-${slot}-error.log`),
		PRIMARY_DOMAIN: primaryDomain
	});

	await mkdir(config.nginxSitesDir, { recursive: true });
	const targetPath = join(config.nginxSitesDir, `${deployment.id}.conf`);
	await writeFile(targetPath, nginxBody, 'utf8');
	await log('info', 'Wrote nginx config', { targetPath, domainList });
}

export async function configureDeploymentIngress({
	deployment,
	domains,
	port,
	slot,
	obtainCertificate = true
}) {
	if (!domains.length) {
		return { tlsEnabled: false, certificateRequested: false };
	}

	const domainNames = domains.map((d) => d.hostname);
	const primaryDomain = domainNames[0];
	const ensureConfig = async (tls) => {
		await writeDeploymentConfig({ deployment, domains, port, slot, tlsEnabled: tls });
		await reloadNginx();
	};

	const hasCertificate = await certificateExists(primaryDomain);
	if (hasCertificate) {
		await ensureConfig(true);
		return { tlsEnabled: true, certificateRequested: false };
	}

	await ensureConfig(false);
	if (!obtainCertificate) {
		return { tlsEnabled: false, certificateRequested: false };
	}

	await requestCertificate(domainNames);
	await ensureConfig(true);
	return { tlsEnabled: true, certificateRequested: true };
}

export async function reloadNginx() {
	await log('info', 'Reloading nginx');
	const process = spawn(['systemctl', 'reload', 'nginx'], {
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const stderr = await new Response(process.stderr).text();
	if (process.exitCode !== 0) {
		throw new Error(`Failed to reload nginx: ${stderr}`);
	}
}
