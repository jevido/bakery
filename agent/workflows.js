import { mkdir, rm, writeFile, access, readdir, unlink, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { nanoid } from 'nanoid';
import { getConfig } from '../app/src/lib/server/config.js';
import { configureDeploymentIngress, reloadNginx } from '../app/src/lib/server/nginx.js';
import {
	stopService,
	startService,
	reloadDaemon,
	enableService,
	serviceNameForDeployment
} from '../app/src/lib/server/systemd.js';
import { startLocalService, stopLocalService } from '../app/src/lib/server/localRuntime.js';
import {
	detectDockerfile,
	buildImage,
	runContainer,
	stopAndRemoveContainer
} from '../app/src/lib/server/docker.js';
import { createLogger } from '../app/src/lib/server/logger.js';
import {
	logDeployment,
	updateDeploymentStatus,
	recordDeploymentVersionRemote,
	getDeploymentContext
} from './apiClient.js';

const logger = createLogger('agent:workflows');

function computeSlot(deployment) {
	const active = deployment.active_slot || 'blue';
	if (!deployment.blue_green_enabled) {
		return { slot: 'blue', port: computePort(deployment.id, 'blue') };
	}
	const slot = active === 'blue' ? 'green' : 'blue';
	return { slot, port: computePort(deployment.id, slot) };
}

function computePort(id, slot) {
	const config = getConfig();
	const hash = Array.from(id).reduce((acc, char) => acc + char.charCodeAt(0), 0);
	const base = config.blueGreenBasePort + (hash % 1000) * 4;
	return slot === 'blue' ? base : base + 1;
}

async function runCommand(command, args, options = {}) {
	await logger.info('Running command', { command, args });
	const process = spawn([command, ...args], {
		cwd: options.cwd,
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const stdout = await new Response(process.stdout).text();
	const stderr = await new Response(process.stderr).text();

	if (process.exitCode !== 0) {
		await logger.error('Command failed', { command, args, stderr });
		throw new Error(stderr || `Command ${command} failed`);
	}
	return stdout;
}

async function cloneRepository({ repository, branch, accessToken, targetDir }) {
	const [owner, repo] = repository.split('/');
	const tokenUrl = accessToken
		? `https://${accessToken}@github.com/${owner}/${repo}.git`
		: `https://github.com/${owner}/${repo}.git`;
	await runCommand('git', ['clone', '--depth', '1', '--branch', branch, tokenUrl, targetDir]);
}

async function installDependencies(dir) {
	await runCommand('bun', ['install'], { cwd: dir });
}

async function fileExists(path) {
	try {
		await access(path);
		return true;
	} catch {
		return false;
	}
}

async function loadPackageScripts(dir) {
	try {
		const packageJson = JSON.parse(await readFile(join(dir, 'package.json'), 'utf8'));
		const scripts = packageJson?.scripts || {};
		return {
			hasBuildScript: Boolean(scripts.build),
			hasStartScript: Boolean(scripts.start)
		};
	} catch {
		return { hasBuildScript: false, hasStartScript: false };
	}
}

async function buildProject(dir) {
	const { hasBuildScript, hasStartScript } = await loadPackageScripts(dir);

	if (hasBuildScript) {
		await runCommand('bun', ['run', 'build'], { cwd: dir });
	} else {
		await logger.info('No build script detected, skipping build');
	}

	const buildEntry = join(dir, 'build', 'index.js');
	const hasBuildOutput = await fileExists(buildEntry);

	return {
		hasBuildScript,
		hasStartScript,
		hasBuildOutput
	};
}

function resolveRuntimeArgs({ hasBuildOutput, hasStartScript }) {
	if (hasBuildOutput) {
		return ['run', 'build/index.js'];
	}
	if (hasStartScript) {
		return ['run', 'start'];
	}
	throw new Error(
		'No production entry point found. Add a build script that outputs build/index.js (recommended) or define a start script.'
	);
}

async function createSystemdUnit({ deployment, slot, port, workingDir, env, runtimeArgs }) {
	const config = getConfig();
	const templatePath = join(
		process.cwd(),
		'infrastructure',
		'systemd',
		'deployments',
		'service.template'
	);
	const template = await Bun.file(templatePath).text();
	const serviceName = serviceNameForDeployment(deployment.id, slot);
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
		.replace('{{WORKING_DIRECTORY}}', workingDir)
		.replace('{{EXEC_START}}', ExecStart)
		.replace('{{ENVIRONMENT}}', envLines);

	const targetPath = join(config.systemdServicesDir, `${serviceName}.service`);
	await writeFile(targetPath, rendered, 'utf8');
	if (!config.localMode) {
		await reloadDaemon();
		await enableService(`${serviceName}.service`);
	}
	return targetPath;
}

async function deployDockerApp({ deployment, slot, port, repoDir, env }) {
	const imageTag = `bakery/${deployment.id}:${slot}`;
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const dockerfilePath = deployment.dockerfile_path || 'Dockerfile';
	const buildContext = deployment.build_context || '.';
	const resolvedDockerfile = join(repoDir, dockerfilePath);
	const resolvedContext = join(repoDir, buildContext);
	await stopAndRemoveContainer(serviceName);
	await buildImage({ context: resolvedContext, tag: imageTag, dockerfile: resolvedDockerfile });
	await runContainer({
		image: imageTag,
		name: serviceName,
		env: {
			...env,
			PORT: String(port)
		},
		portMapping: { host: port, container: env.PORT || 3000 }
	});
}

async function deployBunApp({ deployment, slot, port, repoDir, env, runtimeArgs }) {
	const config = getConfig();
	const serviceName = serviceNameForDeployment(deployment.id, slot);

	if (config.localMode) {
		await startLocalService(serviceName, {
			cwd: repoDir,
			env: {
				...env,
				PORT: String(port)
			},
			args: runtimeArgs,
			command: 'bun',
			onExit: async ({ code, expected }) => {
				if (expected) return;
				if (code === 0) {
					await logDeployment(deployment.id, 'info', 'Node runtime stopped', {
						stream: 'system',
						service: serviceName
					});
					await updateDeploymentStatus(deployment.id, { status: 'inactive' });
					return;
				}
				await logDeployment(
					deployment.id,
					'error',
					`Node runtime crashed (exit code ${code ?? 'unknown'})`,
					{
						stream: 'system',
						service: serviceName
					}
				);
				await updateDeploymentStatus(deployment.id, { status: 'failed' });
			}
		});
		return;
	}

	await createSystemdUnit({
		deployment,
		slot,
		port,
		workingDir: repoDir,
		env: {
			...env,
			PORT: String(port)
		},
		runtimeArgs
	});
	try {
		await stopService(`${serviceName}.service`);
	} catch {}
	await startService(`${serviceName}.service`);
}

async function cleanupOldBuilds(deployment) {
	const config = getConfig();
	const dir = join(config.buildsDir, deployment.id);
	const keep = config.releasesToKeep;
	const entries = await readdir(dir).catch(() => []);
	if (entries.length > keep) {
		const sorted = entries.sort();
		const toRemove = sorted.slice(0, entries.length - keep);
		await Promise.all(
			toRemove.map((entry) => rm(join(dir, entry), { recursive: true, force: true }))
		);
		await logger.info('Cleaned up builds', { deploymentId: deployment.id, removed: toRemove });
	}
}

export async function deployTask({ deploymentId, commitSha }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment, environment, domains, githubAccessToken } = context;
	const { slot, port } = computeSlot(deployment);
	const buildId = nanoid();
	const config = getConfig();
	const deploymentDir = join(config.buildsDir, deployment.id);
	const slotDir = join(deploymentDir, `${slot}-${buildId}`);
	await mkdir(slotDir, { recursive: true });
	const repoDir = join(slotDir, 'source');
	await mkdir(repoDir, { recursive: true });

	const envVars = {
		...environment,
		PORT: String(port)
	};

	await updateDeploymentStatus(deployment.id, { status: 'deploying' });
	await logDeployment(deployment.id, 'info', 'Cloning repository', {});

	await cloneRepository({
		repository: deployment.repository,
		branch: deployment.branch,
		accessToken: githubAccessToken,
		targetDir: repoDir
	});

	const dockerfilePath = deployment.dockerfile_path || 'Dockerfile';
	const buildContext = deployment.build_context || '.';
	const dockerfileExists = await detectDockerfile(repoDir, dockerfilePath);
	if (!dockerfileExists && dockerfilePath && dockerfilePath !== 'Dockerfile') {
		const message = `Dockerfile not found at ${dockerfilePath}`;
		await logDeployment(deployment.id, 'error', message, { path: dockerfilePath });
		throw new Error(message);
	}
	const dockerized = dockerfileExists;
	await logDeployment(deployment.id, 'info', 'Detected project type', {
		dockerized,
		dockerfilePath,
		buildContext
	});
	deployment.dockerized = dockerized;
	deployment.dockerfile_path = dockerfilePath;
	deployment.build_context = buildContext;

	let runtimeArgs;
	if (!dockerized) {
		await installDependencies(repoDir);
		const buildMetadata = await buildProject(repoDir);
		try {
			runtimeArgs = resolveRuntimeArgs(buildMetadata);
		} catch (error) {
			await logDeployment(deployment.id, 'error', error.message, { stream: 'system' });
			throw error;
		}
	}

	if (dockerized) {
		await deployDockerApp({ deployment, slot, port, repoDir, env: envVars });
	} else {
		await deployBunApp({ deployment, slot, port, repoDir, env: envVars, runtimeArgs });
	}

	if (domains.length) {
		const domainNames = domains.map((d) => d.hostname);
		const canRequestCert = Boolean(config.certbotEmail);
		try {
			const ingress = await configureDeploymentIngress({
				deployment,
				domains,
				port,
				slot,
				obtainCertificate: canRequestCert
			});
			if (ingress.certificateRequested) {
				await logDeployment(deployment.id, 'info', 'Issued TLS certificate', {
					domains: domainNames
				});
			}
			if (!ingress.tlsEnabled && !canRequestCert) {
				await logDeployment(
					deployment.id,
					'warn',
					'TLS disabled — CERTBOT_EMAIL is not configured',
					{ domains: domainNames }
				);
			}
		} catch (error) {
			await logDeployment(deployment.id, 'error', 'Failed to configure TLS', {
				error: error.message
			});
			throw error;
		}
	}

	const version = await recordDeploymentVersionRemote(deployment.id, {
		slot,
		commitSha,
		status: 'active',
		port,
		dockerized,
		artifactPath: repoDir
	});

	await updateDeploymentStatus(deployment.id, {
		status: 'running',
		active_slot: slot,
		dockerized
	});

	await logDeployment(deployment.id, 'info', 'Deployment completed', {
		slot,
		port,
		versionId: version?.versionId || version
	});

	await cleanupOldBuilds(deployment);

	return { slot, port };
}

export async function activateVersionTask({ deploymentId, version }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment, domains } = context;
	await configureDeploymentIngress({
		deployment,
		domains,
		port: version.port,
		slot: version.slot,
		obtainCertificate: false
	});
	await updateDeploymentStatus(deployment.id, {
		active_slot: version.slot,
		status: 'running'
	});
	await logDeployment(deployment.id, 'info', 'Activated deployment version', {
		versionId: version.id,
		slot: version.slot
	});
}

export async function cleanupDeploymentTask({ deploymentId }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment } = context;
	const config = getConfig();
	const slots = deployment.blue_green_enabled ? ['blue', 'green'] : ['blue'];
	for (const slot of slots) {
		const serviceName = serviceNameForDeployment(deployment.id, slot);
		if (config.localMode) {
			await stopLocalService(serviceName);
		} else {
			try {
				await stopService(`${serviceName}.service`);
			} catch {}
		}
		try {
			await stopAndRemoveContainer(serviceName);
		} catch {}
		const servicePath = join(config.systemdServicesDir, `${serviceName}.service`);
		await unlink(servicePath).catch(() => {});
	}
	if (!config.localMode) {
		await reloadDaemon().catch(() => {});
	}
	const buildDir = join(config.buildsDir, deployment.id);
	await rm(buildDir, { recursive: true, force: true }).catch(() => {});
	const nginxConfig = join(config.nginxSitesDir, `${deployment.id}.conf`);
	await unlink(nginxConfig).catch(() => {});
	await reloadNginx().catch(() => {});
	await logDeployment(deployment.id, 'info', 'Cleaned up deployment resources', {});
	await updateDeploymentStatus(deployment.id, { status: 'deleted' }).catch(() => {});
}

export async function stopDeploymentRuntimeTask({ deploymentId }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment } = context;
	if (!deployment?.active_slot) {
		throw new Error('Deployment has no active slot to stop');
	}
	const slot = deployment.active_slot;
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const config = getConfig();
	try {
		if (config.localMode) {
			await stopLocalService(serviceName);
		} else {
			await stopService(`${serviceName}.service`);
		}
	} catch (error) {
		await logDeployment(deployment.id, 'error', 'Failed to stop deployment runtime', {
			error: error.message,
			service: serviceName,
			slot
		});
		throw error;
	}
	await updateDeploymentStatus(deployment.id, { status: 'inactive' });
	await logDeployment(deployment.id, 'info', 'Deployment stopped', { slot, service: serviceName });
}

export async function startDeploymentRuntimeTask({ deploymentId }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment } = context;
	if (!deployment?.active_slot) {
		throw new Error('Deployment has no active slot to start');
	}
	const slot = deployment.active_slot;
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const config = getConfig();
	if (config.localMode) {
		throw new Error('Start command is unavailable in local mode. Redeploy instead.');
	}
	try {
		await startService(`${serviceName}.service`);
	} catch (error) {
		await logDeployment(deployment.id, 'error', 'Failed to start deployment runtime', {
			error: error.message,
			service: serviceName,
			slot
		});
		throw error;
	}
	await updateDeploymentStatus(deployment.id, { status: 'running' });
	await logDeployment(deployment.id, 'info', 'Deployment resumed', {
		slot,
		service: serviceName
	});
}
