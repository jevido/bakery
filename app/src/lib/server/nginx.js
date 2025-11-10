import { readFile, writeFile, mkdir, access } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';
import { certificateExists, requestCertificate } from './certbot.js';
import { shouldSkipTls } from './domainUtils.js';

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

export async function writeDeploymentConfig({ deployment, domains, port, slot, tlsEnabled }) {
	const config = getConfig();
	const skipTls = shouldSkipTls(domains, config);
	const domainList = domains.map((d) => d.hostname).join(' ');
	const primaryDomain = domains[0] ? domains[0].hostname : `${deployment.id}.local`;
	const httpsDomains = domains.map((d) => `server_name ${d.hostname};`).join('\n  ');
	const enableTls =
		!skipTls && (typeof tlsEnabled === 'boolean' ? tlsEnabled : await certificateExists(primaryDomain));
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
	const config = getConfig();
	const skipTls = shouldSkipTls(domains, config);
	if (!domains.length) {
		return { tlsEnabled: false, certificateRequested: false };
	}

	const domainNames = domains.map((d) => d.hostname);
	const primaryDomain = domainNames[0];
	const ensureConfig = async (tls) => {
		await writeDeploymentConfig({ deployment, domains, port, slot, tlsEnabled: tls });
		await reloadNginx();
	};

	if (skipTls) {
		await ensureConfig(false);
		return { tlsEnabled: false, certificateRequested: false, skippedTls: true };
	}

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

async function runCommand(command, args) {
	let child;
	const PATH_FALLBACK = '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin';
	const envPath = process.env.PATH ? `${process.env.PATH}:${PATH_FALLBACK}` : PATH_FALLBACK;
	try {
		child = spawn([command, ...args], {
			stdin: 'ignore',
			stdout: 'pipe',
			stderr: 'pipe',
			env: {
				...process.env,
				PATH: envPath
			}
		});
	} catch (error) {
		if (error?.code === 'ENOENT') {
			return { exitCode: 127, stdout: '', stderr: error.message, enoent: true };
		}
		throw error;
	}
	const [stdout, stderr] = await Promise.all([
		child.stdout ? new Response(child.stdout).text() : '',
		child.stderr ? new Response(child.stderr).text() : ''
	]);
	const exitCode = await child.exited;
	return { exitCode, stdout, stderr, enoent: false };
}

export async function reloadNginx() {
	const config = getConfig();
	if (config.localMode) {
		await log('info', 'Local mode: skipping nginx reload');
		return;
	}

	const candidates = [
		config.nginxExecutable,
		'/usr/sbin/nginx',
		'/usr/local/sbin/nginx',
		'/usr/bin/nginx',
		'nginx'
	].filter(Boolean);
	await log('info', 'Validating nginx configuration');
	let nginxExecutable = null;
	for (const candidate of candidates) {
		const result = await runCommand(candidate, ['-t']);
		if (result.enoent) {
			await log('warn', 'nginx executable missing', { candidate });
			continue;
		}
		if (result.exitCode !== 0) {
			const details = `${result.stdout}${result.stderr}`.trim();
			await log('error', 'nginx -t failed', { output: details, executable: candidate });
			throw new Error(`nginx config test failed: ${details}`);
		}
		nginxExecutable = candidate;
		break;
	}
	if (!nginxExecutable) {
		throw new Error(`nginx executable not found. Tried ${candidates.join(', ')}`);
	}

	await log('info', 'Reloading nginx');
	const reload = await runCommand('systemctl', ['reload', 'nginx']);
	if (reload.exitCode !== 0) {
		const details = reload.stderr.trim();
		throw new Error(`Failed to reload nginx: ${details}`);
	}
}
