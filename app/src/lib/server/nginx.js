import { readFile, writeFile, mkdir } from 'node:fs/promises';
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
	const listenDirective = enableTls ? 'listen 443 ssl http2;' : 'listen 80;';
	const sslDirectives = enableTls
		? [
			`    ssl_certificate /etc/letsencrypt/live/${primaryDomain}/fullchain.pem;`,
			`    ssl_certificate_key /etc/letsencrypt/live/${primaryDomain}/privkey.pem;`,
			'    include /etc/letsencrypt/options-ssl-nginx.conf;',
			'    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;'
		  ].join('\n')
		: '    # TLS disabled until a certificate is available\n';

	const nginxBody = await renderTemplate('app.conf', {
		UPSTREAM_NAME: upstreamName,
		PORT: port,
		HTTPS_DOMAINS: httpsDomains,
		HTTP_REDIRECT_BLOCKS: httpRedirects,
		LISTEN_DIRECTIVE: listenDirective,
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
