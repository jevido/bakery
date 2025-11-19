import { posix as path } from 'node:path';
import { join } from 'node:path';
import { nanoid } from 'nanoid';
import { getConfig } from './config.js';
import { createSshRunner, shellEscape } from './sshClient.js';
import { exportEnvVars } from './models/envModel.js';
import {
	recordDeploymentVersion,
	updateDeployment,
	recordDeploymentLog
} from './models/deploymentModel.js';
import { listDomains } from './models/domainModel.js';
import { getNodeWithCredentials } from './models/nodeModel.js';
import { shouldSkipTls } from './domainUtils.js';
import { serviceNameForDeployment } from './systemd.js';
import {
	computeSlot,
	resolveRuntimeArgs,
	ensureControllableSlot,
	dockerImageTag
} from './deployment/utils.js';

const SYSTEMCTL_PATH = '/usr/bin/systemctl';
const CERTBOT_PATH = '/usr/bin/certbot';

async function getDeploymentNode(deployment) {
	if (!deployment.node_id) {
		throw new Error('Deployment is not assigned to a node');
	}
	const node = await getNodeWithCredentials(deployment.node_id);
	if (!node?.ssh_host || !node?.ssh_private_key) {
		throw new Error('Node is missing SSH configuration');
	}
	return node;
}

async function withRunner(node, deploymentId, handler) {
	const runner = await createSshRunner(
		{
			host: node.ssh_host,
			port: node.ssh_port || 22,
			user: node.ssh_user || 'bakery-agent',
			privateKey: node.ssh_private_key
		},
		{
			onCommandStart: (command) =>
				recordDeploymentLog(deploymentId, 'info', `$ ${command}`, {
					stream: 'system',
					nodeId: node.id
				}),
			onStdout: (line) =>
				recordDeploymentLog(deploymentId, 'info', line, {
					stream: 'stdout',
					nodeId: node.id
				}),
			onStderr: (line) =>
				recordDeploymentLog(deploymentId, 'error', line, {
					stream: 'stderr',
					nodeId: node.id
				})
		}
	);
	try {
		return await handler(runner);
	} finally {
		await runner.dispose();
	}
}

function buildRepositoryUrl(repository, branch, accessToken) {
	const [owner, repo] = repository.split('/');
	if (accessToken) {
		return `https://${accessToken}@github.com/${owner}/${repo}.git`;
	}
	return `https://github.com/${owner}/${repo}.git`;
}

async function cleanupRemoteBuilds(runner, deployment) {
	const config = getConfig();
	const keep = config.releasesToKeep;
	const buildsPath = path.join(config.nodeBuildsDir, deployment.id);
	const cleanupScript = `
if [ ! -d ${shellEscape(buildsPath)} ]; then
  exit 0
fi
cd ${shellEscape(buildsPath)}
count=$(ls -1 | wc -l)
if [ "$count" -le ${keep} ]; then
  exit 0
fi
ls -1t | tail -n +$(( ${keep} + 1 )) | while read entry; do
  rm -rf "$entry"
done
`.trim();
	await runner.exec(cleanupScript, { log: false });
}

function sanitizeOutput(value) {
	if (!value) return null;
	const trimmed = value.trim();
	if (!trimmed) return null;
	const filtered = trimmed
		.split('\n')
		.filter((line) => !/^[A-Z0-9_]+\s*=\s*.*$/.test(line.trim()))
		.join('\n')
		.trim();
	if (!filtered) {
		return null;
	}
	const limit = 2000;
	return filtered.length > limit ? `${filtered.slice(-limit)} (truncated)` : filtered;
}

async function deployDockerAppOnNode({ runner, deployment, slot, port, repoDir, env }) {
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const imageTag = dockerImageTag(deployment.id, slot);
	const dockerfilePath = path.join(repoDir, deployment.dockerfile_path || 'Dockerfile');
	const buildContext = path.join(repoDir, deployment.build_context || '.');
	const cleanupCommand = `
if docker ps -a --format '{{.Names}}' | grep -qw ${shellEscape(serviceName)}; then
  docker rm -f ${shellEscape(serviceName)};
fi
`.trim();
	await runner.exec(cleanupCommand, { log: false });
	await runner.exec(
		`DOCKER_CLI_HINTS=0 docker build -t ${shellEscape(imageTag)} -f ${shellEscape(
			dockerfilePath
		)} ${shellEscape(buildContext)}`
	);
	const envArgs = Object.entries(env)
		.map(([key, value]) => `-e ${shellEscape(`${key}=${value}`)}`)
		.join(' ');
	const envSegment = envArgs ? `${envArgs} ` : '';
	const containerPort = env.PORT || 3000;
	const runCmd = `docker run -d --name ${shellEscape(
		serviceName
	)} --restart always ${envSegment}-p ${port}:${containerPort} ${shellEscape(imageTag)}`;
	await runner.exec(runCmd);
	return { serviceName };
}

async function deployBunAppOnNode({ runner, deployment, slot, port, repoDir, env, runtimeArgs }) {
	const config = getConfig();
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const templatePath = join(
		process.cwd(),
		'infrastructure',
		'systemd',
		'deployments',
		'service.template'
	);
	const template = await Bun.file(templatePath).text();
	const argString = (runtimeArgs || ['run', 'start'])
		.map((arg) => (/\s/.test(arg) ? `"${arg.replace(/"/g, '\\"')}"` : arg))
		.join(' ');
	const ExecStart = deployment.dockerized
		? `/usr/bin/docker start -a ${serviceName}`
		: `/usr/bin/env PORT=${port} bun ${argString}`;
	const envLines = Object.entries(env)
		.map(([key, value]) => `Environment="${key}=${String(value).replace(/"/g, '\\"')}"`)
		.join('\n');
	const rendered = template
		.replace('{{SERVICE_NAME}}', serviceName)
		.replace('{{WORKING_DIRECTORY}}', repoDir)
		.replace('{{EXEC_START}}', ExecStart)
		.replace('{{ENVIRONMENT}}', envLines);
	const servicePath = path.join(config.nodeSystemdDir, `${serviceName}.service`);
	await runner.writeFile(servicePath, rendered, { sudo: true });
	await runner.exec(`sudo -n ${SYSTEMCTL_PATH} daemon-reload`);
	await runner.exec(`sudo -n ${SYSTEMCTL_PATH} enable ${shellEscape(`${serviceName}.service`)}`);
	await runner.exec(`sudo -n ${SYSTEMCTL_PATH} stop ${shellEscape(`${serviceName}.service`)}`, {
		acceptExitCodes: [0, 5],
		log: false
	});
	await runner.exec(`sudo -n ${SYSTEMCTL_PATH} start ${shellEscape(`${serviceName}.service`)}`);
	return { serviceName, servicePath };
}

async function loadRemoteTemplate(runner) {
	const remotePath = '/var/lib/bakery-node/templates/nginx/app.conf';
	try {
		return await runner.readFile(remotePath, { log: false });
	} catch {
		return null;
	}
}

async function renderNodeNginxBody({
	runner,
	deployment,
	domains,
	port,
	slot,
	enableTls,
	extraSslDirectives = []
}) {
	const config = getConfig();
	const logsDir = config.nodeLogsDir || config.logsDir;
	const remoteTemplate = await loadRemoteTemplate(runner);
	const template =
		remoteTemplate ?? (await Bun.file(join(config.nginxTemplateDir, 'app.conf')).text());
	const domainList = domains.map((d) => d.hostname).join(' ');
	const primaryDomain = domains[0] ? domains[0].hostname : `${deployment.id}.local`;
	const httpsDomains = domains.map((d) => `server_name ${d.hostname};`).join('\n  ');
	const httpRedirects = enableTls
		? domains
				.map(
					(d) => `
server {
  listen 80;
  server_name ${d.hostname};
  return 301 https://${d.hostname}$request_uri;
}
`.trim()
				)
				.join('\n\n')
		: '';
	let listenDirective = 'listen 80;';
	let http2Directive = '# http/1.1 only';
	let sslDirectives = '    # TLS disabled until a certificate is available\n';
	if (enableTls) {
		listenDirective = 'listen 443 ssl http2;';
		http2Directive = '# HTTP/2 enabled via listen directive';
		const directives = [
			`    ssl_certificate /etc/letsencrypt/live/${primaryDomain}/fullchain.pem;`,
			`    ssl_certificate_key /etc/letsencrypt/live/${primaryDomain}/privkey.pem;`
		];
		extraSslDirectives.forEach((line) => directives.push(`    ${line}`));
		sslDirectives = directives.join('\n');
	}
	return interpolateTemplate(template, {
		UPSTREAM_NAME: `bakery_${deployment.id}_${slot}`,
		PORT: port,
		HTTPS_DOMAINS: httpsDomains,
		HTTP_REDIRECT_BLOCKS: httpRedirects,
		LISTEN_DIRECTIVE: listenDirective,
		HTTP2_DIRECTIVE: http2Directive,
		SSL_DIRECTIVES: sslDirectives,
		ACCESS_LOG: path.join(logsDir, `${deployment.id}-${slot}-access.log`),
		ERROR_LOG: path.join(logsDir, `${deployment.id}-${slot}-error.log`),
		PRIMARY_DOMAIN: domainList || primaryDomain
	});
}

async function writeNodeNginxConfig({
	runner,
	deployment,
	domains,
	port,
	slot,
	enableTls
}) {
	const config = getConfig();
	let extraSslDirectives = [];
	if (enableTls) {
		const optionsExists = await runner.fileExists('/etc/letsencrypt/options-ssl-nginx.conf', {
			log: false
		});
		const dhparamExists = await runner.fileExists('/etc/letsencrypt/ssl-dhparams.pem', {
			log: false
		});
		if (optionsExists) {
			extraSslDirectives.push('include /etc/letsencrypt/options-ssl-nginx.conf;');
		}
		if (dhparamExists) {
			extraSslDirectives.push('ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;');
		}
	}
	const body = await renderNodeNginxBody({
		runner,
		deployment,
		domains,
		port,
		slot,
		enableTls,
		extraSslDirectives
	});
	const targetPath = path.join(config.nodeNginxSitesDir, `${deployment.id}.conf`);
	await runner.writeFile(targetPath, body, { sudo: true });
	await ensureRemoteNginxConfigIsValid(runner, deployment);
	await reloadRemoteNginx(runner, deployment);
}

async function ensureRemoteNginxConfigIsValid(runner, deployment) {
	const config = getConfig();
	const candidates = [
		config.nginxExecutable,
		'/usr/sbin/nginx',
		'/usr/local/sbin/nginx',
		'/usr/bin/nginx',
		'nginx'
	].filter(Boolean);
	for (const candidate of candidates) {
		try {
			await runner.exec(`sudo -n ${shellEscape(candidate)} -t`, { log: false });
			return;
		} catch (error) {
			if (error?.exitCode === 127) {
				continue;
			}
			const output = sanitizeOutput(`${error?.stdout || ''}\n${error?.stderr || ''}`);
			const message = output
				? `nginx config test failed (${candidate}): ${output}`
				: `nginx config test failed when running ${candidate} -t`;
			await recordDeploymentLog(deployment.id, 'error', message, {
				stream: 'system',
				executable: candidate
			});
			throw new Error(message);
		}
	}
	const message = `nginx executable not found on remote node (tried ${candidates.join(', ')})`;
	await recordDeploymentLog(deployment.id, 'error', message, { stream: 'system' });
	throw new Error(message);
}

async function reloadRemoteNginx(runner, deployment) {
	try {
		await runner.exec(`sudo -n ${SYSTEMCTL_PATH} reload nginx`);
	} catch (error) {
		try {
			const status = await runner.exec(`sudo -n ${SYSTEMCTL_PATH} status nginx --no-pager`, {
				log: false,
				acceptExitCodes: [0, 3, 4, 5],
				strict: false
			});
			const output = sanitizeOutput(`${status.stdout || ''}\n${status.stderr || ''}`);
			if (output) {
				await recordDeploymentLog(deployment.id, 'error', 'nginx reload status', {
					stream: 'system',
					output
				});
			}
		} catch {
			// ignore secondary failures
		}
		throw error;
	}
}

async function remoteCertificateExists(runner, domain) {
	if (!domain) return false;
	const base = `/etc/letsencrypt/live/${domain}`;
	const result = await runner.exec(
		`[ -f ${shellEscape(`${base}/fullchain.pem`)} ] && [ -f ${shellEscape(`${base}/privkey.pem`)} ]`,
		{ acceptExitCodes: [0, 1], log: false }
	);
	return result.exitCode === 0;
}

async function requestRemoteCertificate(runner, domains) {
	const config = getConfig();
	if (!domains.length || !config.certbotEmail) return false;
	const args = [
		'certonly',
		'--standalone',
		'--keep-until-expiring',
		'--expand',
		'--agree-tos',
		'--non-interactive',
		'--email',
		config.certbotEmail,
		'--cert-name',
		domains[0],
		...domains.flatMap((domain) => ['-d', domain])
	].map((arg) => shellEscape(arg));
	const command = ['sudo', '-n', CERTBOT_PATH, ...args].join(' ');
	await runner.exec(`sudo -n ${SYSTEMCTL_PATH} stop nginx`, { acceptExitCodes: [0, 5] });
	try {
		await runner.exec(command, { strict: false });
	} finally {
		await runner.exec(`sudo -n ${SYSTEMCTL_PATH} start nginx`, { acceptExitCodes: [0, 5] });
	}
	return true;
}

async function configureNodeIngress({
	runner,
	deployment,
	domains,
	port,
	slot,
	obtainCertificate = true
}) {
	const config = getConfig();
	if (!domains.length) {
		return { tlsEnabled: false, certificateRequested: false };
	}
	const skipTls = shouldSkipTls(domains, config);
	const domainNames = domains.map((d) => d.hostname);
	const primaryDomain = domainNames[0];
	if (skipTls) {
		await writeNodeNginxConfig({ runner, deployment, domains, port, slot, enableTls: false });
		return { tlsEnabled: false, certificateRequested: false, skippedTls: true };
	}
	let hasCertificate = await remoteCertificateExists(runner, primaryDomain);
	await writeNodeNginxConfig({
		runner,
		deployment,
		domains,
		port,
		slot,
		enableTls: hasCertificate
	});
	let certificateRequested = false;
	if (!hasCertificate && obtainCertificate) {
		const requested = await requestRemoteCertificate(runner, domainNames);
		if (requested) {
			certificateRequested = true;
			hasCertificate = true;
			await recordDeploymentLog(deployment.id, 'info', 'Issued TLS certificate', {
				stream: 'system',
				domains: domainNames
			});
		}
	}
	if (!hasCertificate && !config.certbotEmail) {
		await recordDeploymentLog(
			deployment.id,
			'warn',
			'TLS disabled — CERTBOT_EMAIL is not configured',
			{ stream: 'system', domains: domainNames }
		);
	}
	if (hasCertificate) {
		await writeNodeNginxConfig({
			runner,
			deployment,
			domains,
			port,
			slot,
			enableTls: true
		});
	}
	return { tlsEnabled: hasCertificate, certificateRequested };
}

export async function deployToNode(deployment, { commitSha, accessToken } = {}) {
	const node = await getDeploymentNode(deployment);
	await recordDeploymentLog(deployment.id, 'info', 'Starting SSH deployment', {
		stream: 'system',
		nodeId: node.id
	});
	return withRunner(node, deployment.id, async (runner) => {
		try {
			const config = getConfig();
			const { slot, port } = computeSlot(deployment);
			const buildId = nanoid();
			const deploymentDir = path.join(config.nodeBuildsDir, deployment.id);
			const slotDir = path.join(deploymentDir, `${slot}-${buildId}`);
			const repoDir = path.join(slotDir, 'source');
			await runner.exec(`mkdir -p ${shellEscape(repoDir)}`);
			await updateDeployment(deployment.id, { status: 'deploying' });
			await recordDeploymentLog(deployment.id, 'info', 'Cloning repository', { stream: 'system' });
			const cloneUrl = buildRepositoryUrl(deployment.repository, deployment.branch, accessToken);
			await runner.exec(
				`git clone --depth 1 --branch ${shellEscape(deployment.branch)} ${shellEscape(cloneUrl)} ${shellEscape(
					repoDir
				)}`
			);

			const dockerfilePath = deployment.dockerfile_path || 'Dockerfile';
			const buildContext = deployment.build_context || '.';
			const dockerfileExists = await runner.fileExists(path.join(repoDir, dockerfilePath));
			if (!dockerfileExists && dockerfilePath !== 'Dockerfile') {
				const message = `Dockerfile not found at ${dockerfilePath}`;
				await recordDeploymentLog(deployment.id, 'error', message, { stream: 'system' });
				throw new Error(message);
			}
			const dockerized = dockerfileExists;
			await recordDeploymentLog(deployment.id, 'info', 'Detected project type', {
				stream: 'system',
				dockerized,
				dockerfilePath,
				buildContext
			});
			deployment.dockerized = dockerized;
			deployment.dockerfile_path = dockerfilePath;
			deployment.build_context = buildContext;

			const envVars = await exportEnvVars(deployment.id);
			envVars.PORT = String(port);

			let runtimeArgs;
			if (!dockerized) {
				await runner.exec('bun install', { cwd: repoDir });
				let packageJson = {};
				try {
					const packageContents = await runner.readFile(path.join(repoDir, 'package.json'));
					packageJson = JSON.parse(packageContents);
				} catch {
					packageJson = {};
				}
				const scripts = packageJson.scripts || {};
				const hasBuildScript = Boolean(scripts.build);
				const hasStartScript = Boolean(scripts.start);
				if (hasBuildScript) {
					await runner.exec('bun run build', { cwd: repoDir });
				} else {
					await recordDeploymentLog(
						deployment.id,
						'info',
						'No build script detected, skipping bun run build',
						{ stream: 'system' }
					);
				}
				const hasBuildOutput = await runner.fileExists(path.join(repoDir, 'build', 'index.js'));
				try {
					runtimeArgs = resolveRuntimeArgs({ hasBuildOutput, hasStartScript });
				} catch (error) {
					await recordDeploymentLog(deployment.id, 'error', error.message, { stream: 'system' });
					throw error;
				}
			}

			if (dockerized) {
				await deployDockerAppOnNode({ runner, deployment, slot, port, repoDir, env: envVars });
			} else {
				await deployBunAppOnNode({
					runner,
					deployment,
					slot,
					port,
					repoDir,
					env: envVars,
					runtimeArgs
				});
			}

			const domains = await listDomains(deployment.id);
			if (domains.length) {
				try {
					await configureNodeIngress({
						runner,
						deployment,
						domains,
						port,
						slot,
						obtainCertificate: true
					});
				} catch (error) {
					await recordDeploymentLog(deployment.id, 'error', 'Failed to configure ingress', {
						stream: 'system',
						error: error.message
					});
					throw error;
				}
			}

			const versionId = await recordDeploymentVersion({
				deploymentId: deployment.id,
				slot,
				commitSha,
				status: 'active',
				port,
				dockerized,
				artifactPath: repoDir
			});

			await updateDeployment(deployment.id, {
				status: 'running',
				active_slot: slot,
				dockerized
			});

			await recordDeploymentLog(deployment.id, 'info', 'Deployment completed', {
				stream: 'system',
				slot,
				port,
				versionId
			});

			await cleanupRemoteBuilds(runner, deployment);

			return { slot, port, versionId };
		} catch (error) {
			await recordDeploymentLog(deployment.id, 'error', 'Deployment command failed', {
				stream: 'system',
				error: error?.message,
				command: error?.command,
				exitCode: error?.exitCode,
				stdout: sanitizeOutput(error?.stdout),
				stderr: sanitizeOutput(error?.stderr)
			});
			throw error;
		}
	});
}

export async function activateRemoteVersion(deployment, version) {
	const node = await getDeploymentNode(deployment);
	const domains = await listDomains(deployment.id);
	return withRunner(node, deployment.id, async (runner) => {
		await configureNodeIngress({
			runner,
			deployment,
			domains,
			port: version.port,
			slot: version.slot,
			obtainCertificate: false
		});
		await updateDeployment(deployment.id, {
			active_slot: version.slot,
			status: 'running'
		});
		await recordDeploymentLog(deployment.id, 'info', 'Activated deployment version', {
			stream: 'system',
			versionId: version.id,
			slot: version.slot
		});
	});
}

export async function cleanupRemoteDeployment(deployment) {
	const node = await getDeploymentNode(deployment);
	return withRunner(node, deployment.id, async (runner) => {
		const config = getConfig();
		const slots = deployment.blue_green_enabled ? ['blue', 'green'] : ['blue'];
		for (const slot of slots) {
			const serviceName = serviceNameForDeployment(deployment.id, slot);
			await runner
				.exec(`sudo -n ${SYSTEMCTL_PATH} stop ${shellEscape(`${serviceName}.service`)}`, {
					acceptExitCodes: [0, 5],
					log: false
				})
				.catch(() => {});
			await runner
				.exec(`sudo -n rm -f ${shellEscape(path.join(config.nodeSystemdDir, `${serviceName}.service`))}`, {
					acceptExitCodes: [0, 1],
					log: false
				})
				.catch(() => {});
			await runner
				.exec(`docker rm -f ${shellEscape(serviceName)} >/dev/null 2>&1 || true`, {
					log: false,
					strict: false
				})
				.catch(() => {});
		}
			await runner.exec(`sudo -n ${SYSTEMCTL_PATH} daemon-reload`, { acceptExitCodes: [0, 1], log: false });
		await runner
			.exec(`rm -rf ${shellEscape(path.join(config.nodeBuildsDir, deployment.id))}`, {
				acceptExitCodes: [0, 1],
				log: false
			})
			.catch(() => {});
		await runner
			.exec(`sudo -n rm -f ${shellEscape(path.join(config.nodeNginxSitesDir, `${deployment.id}.conf`))}`, {
				acceptExitCodes: [0, 1],
				log: false
			})
			.catch(() => {});
		await runner.exec(`sudo -n ${SYSTEMCTL_PATH} reload nginx`, {
			acceptExitCodes: [0, 1],
			log: false
		}).catch(() => {});
		await recordDeploymentLog(deployment.id, 'info', 'Cleaned up deployment resources', {
			stream: 'system',
			nodeId: node.id
		});
		await updateDeployment(deployment.id, { status: 'deleted' }).catch(() => {});
	});
}

export async function stopRemoteDeployment(deployment) {
	const node = await getDeploymentNode(deployment);
	const slot = ensureControllableSlot(deployment);
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	return withRunner(node, deployment.id, async (runner) => {
		if (deployment.dockerized) {
			await runner.exec(`docker stop ${shellEscape(serviceName)}`);
		} else {
		await runner.exec(`sudo -n ${SYSTEMCTL_PATH} stop ${shellEscape(`${serviceName}.service`)}`);   
		}
		await updateDeployment(deployment.id, { status: 'inactive' });
		await recordDeploymentLog(deployment.id, 'info', 'Deployment stopped', {
			stream: 'system',
			slot,
			service: serviceName
		});
	});
}

export async function startRemoteDeployment(deployment) {
	const node = await getDeploymentNode(deployment);
	const slot = ensureControllableSlot(deployment);
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	return withRunner(node, deployment.id, async (runner) => {
		if (deployment.dockerized) {
			await runner.exec(`docker start ${shellEscape(serviceName)}`);
		} else {
			await runner.exec(`sudo -n ${SYSTEMCTL_PATH} start ${shellEscape(`${serviceName}.service`)}`);
		}
		await updateDeployment(deployment.id, { status: 'running' });
		await recordDeploymentLog(deployment.id, 'info', 'Deployment resumed', {
			stream: 'system',
			slot,
			service: serviceName
		});
	});
}
function interpolateTemplate(template, values) {
	let result = template;
	for (const [key, value] of Object.entries(values)) {
		result = result.replaceAll(`{{${key}}}`, String(value));
	}
	return result;
}
