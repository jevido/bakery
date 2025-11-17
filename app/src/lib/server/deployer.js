import { mkdir, rm, writeFile, access, readdir, unlink, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { TextDecoder } from 'node:util';
import { spawn } from 'bun';
import { nanoid } from 'nanoid';
import { getConfig } from './config.js';
import { getBunExecutable } from './bunPaths.js';
import { sql } from 'bun';
import { exportEnvVars } from './models/envModel.js';
import {
	recordDeploymentVersion,
	updateDeployment,
	recordDeploymentLog
} from './models/deploymentModel.js';
import { listDomains } from './models/domainModel.js';
import { configureDeploymentIngress, reloadNginx } from './nginx.js';
import {
	stopService,
	startService,
	reloadDaemon,
	enableService,
	serviceNameForDeployment
} from './systemd.js';
import { detectDockerfile, buildImage, runContainer, stopAndRemoveContainer } from './docker.js';
import { createLogger } from './logger.js';
import { startLocalService, stopLocalService } from './localRuntime.js';

const logger = createLogger('deployer');
const bunExecutable = getBunExecutable();
const chunkDecoder = new TextDecoder();

async function streamProcessOutput(stream, { deploymentId, streamLabel }) {
	if (!stream) return '';
	if (!deploymentId) {
		return new Response(stream).text();
	}
	const reader = stream.getReader();
	let buffer = '';
	let output = '';
	while (true) {
		const { value, done } = await reader.read();
		if (done) break;
		const chunk = chunkDecoder.decode(value);
		output += chunk;
		buffer += chunk;
		const lines = buffer.split(/\r?\n/);
		buffer = lines.pop() ?? '';
		for (const line of lines) {
			const normalized = line.replace(/\r/g, '');
			if (!normalized.trim()) continue;
			await recordDeploymentLog(
				deploymentId,
				streamLabel === 'stderr' ? 'error' : 'info',
				normalized,
				{ stream: streamLabel }
			);
		}
	}
	const remaining = buffer.replace(/\r/g, '').trim();
	if (remaining) {
		await recordDeploymentLog(
			deploymentId,
			streamLabel === 'stderr' ? 'error' : 'info',
			remaining,
			{ stream: streamLabel }
		);
	}
	return output;
}

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
	const { cwd, deploymentId } = options;
	await logger.info('Running command', {
		command,
		args,
		cwd,
		...(deploymentId ? { deploymentId } : {})
	});
	if (deploymentId) {
		await recordDeploymentLog(deploymentId, 'info', `$ ${command} ${args.join(' ')}`, {
			stream: 'system',
			command,
			args,
			cwd
		});
	}
	const child = spawn([command, ...args], {
		cwd,
		stdout: 'pipe',
		stderr: 'pipe',
		env: {
			...process.env,
			GIT_TERMINAL_PROMPT: '0'
		}
	});
	const [stdout, stderr] = await Promise.all([
		streamProcessOutput(child.stdout, { deploymentId, streamLabel: 'stdout' }),
		streamProcessOutput(child.stderr, { deploymentId, streamLabel: 'stderr' })
	]);
	await child.exited;

	if (child.exitCode !== 0) {
		await logger.error('Command failed', {
			command,
			args,
			stderr,
			cwd,
			...(deploymentId ? { deploymentId } : {})
		});
		if (deploymentId) {
			await recordDeploymentLog(deploymentId, 'error', `Command failed: ${command}`, {
				stream: 'system',
				command,
				args,
				cwd,
				stdout,
				stderr
			});
		}
		throw new Error(stderr || `Command ${command} failed`);
	}

	if (deploymentId && stdout.trim()) {
		await recordDeploymentLog(deploymentId, 'info', `${command} completed`, {
			stream: 'system',
			command,
			args,
			cwd,
			stdout
		});
	}

	return stdout;
}

async function cloneRepository({ repository, branch, accessToken, targetDir, deploymentId }) {
	const [owner, repo] = repository.split('/');
	const tokenUrl = accessToken
		? `https://${accessToken}@github.com/${owner}/${repo}.git`
		: `https://github.com/${owner}/${repo}.git`;
	await runCommand('git', ['clone', '--depth', '1', '--branch', branch, tokenUrl, targetDir], {
		cwd: undefined,
		deploymentId
	});
}

async function installDependencies(dir, deploymentId) {
	await runCommand(bunExecutable, ['install'], { cwd: dir, deploymentId });
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

async function buildProject(dir, deploymentId) {
	const { hasBuildScript, hasStartScript } = await loadPackageScripts(dir);

	if (hasBuildScript) {
		await runCommand(bunExecutable, ['run', 'build'], { cwd: dir, deploymentId });
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
	const quotedBun = bunExecutable.includes(' ')
		? `"${bunExecutable}"`
		: bunExecutable;
	const argString = (runtimeArgs || ['run', 'start'])
		.map((arg) => (/\s/.test(arg) ? `"${arg.replace(/"/g, '\\"')}"` : arg))
		.join(' ');
	const ExecStart = deployment.dockerized
		? `/usr/bin/docker start -a ${serviceName}`
		: `/usr/bin/env PORT=${port} ${quotedBun} ${argString}`;

	const envLines = Object.entries(env)
		.map(([key, value]) => `Environment="${key}=${value.replace(/"/g, '\\"')}"`)
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
			command: bunExecutable,
			args: runtimeArgs,
			onExit: async ({ code, expected }) => {
				if (expected) return;
				if (code === 0) {
					await recordDeploymentLog(deployment.id, 'info', 'Local runtime stopped', {
						stream: 'system',
						service: serviceName
					});
					await updateDeployment(deployment.id, { status: 'inactive' });
					return;
				}
				await recordDeploymentLog(
					deployment.id,
					'error',
					`Local runtime crashed (exit code ${code ?? 'unknown'})`,
					{
						stream: 'system',
						service: serviceName
					}
				);
				await updateDeployment(deployment.id, { status: 'failed' });
			}
		});
		return null;
	}

	const servicePath = await createSystemdUnit({
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
	return servicePath;
}

export async function deploy(deployment, options) {
	const config = getConfig();
	const { slot, port } = computeSlot(deployment);
	const buildId = nanoid();
	const deploymentDir = join(config.buildsDir, deployment.id);
	const slotDir = join(deploymentDir, `${slot}-${buildId}`);
	await mkdir(slotDir, { recursive: true });
	const repoDir = join(slotDir, 'source');

	const envVars = await exportEnvVars(deployment.id);
	envVars.PORT = String(port);

	const accessToken = options.accessToken || null;
	await recordDeploymentLog(deployment.id, 'info', 'Cloning repository', { stream: 'system' });
	await cloneRepository({
		repository: deployment.repository,
		branch: deployment.branch,
		accessToken,
		targetDir: repoDir,
		deploymentId: deployment.id
	});

	const dockerfilePath = deployment.dockerfile_path || 'Dockerfile';
	const buildContext = deployment.build_context || '.';
	const dockerfileExists = await detectDockerfile(repoDir, dockerfilePath);
	if (!dockerfileExists && dockerfilePath && dockerfilePath !== 'Dockerfile') {
		const message = `Dockerfile not found at ${dockerfilePath}`;
		await recordDeploymentLog(deployment.id, 'error', message, {
			stream: 'system',
			path: dockerfilePath
		});
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

	let runtimeArgs;
	if (!dockerized) {
		await installDependencies(repoDir, deployment.id);
		const buildMetadata = await buildProject(repoDir, deployment.id);
		try {
			runtimeArgs = resolveRuntimeArgs(buildMetadata);
		} catch (error) {
			await recordDeploymentLog(deployment.id, 'error', error.message, { stream: 'system' });
			throw error;
		}
	}

	if (dockerized) {
		await deployDockerApp({ deployment, slot, port, repoDir, env: envVars });
	} else {
		await deployBunApp({
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
		await recordDeploymentLog(deployment.id, 'info', 'Issued TLS certificate', {
			stream: 'system',
			domains: domainNames
		});
			}
			if (!ingress.tlsEnabled && !canRequestCert) {
			await recordDeploymentLog(
				deployment.id,
				'warn',
				'TLS disabled — CERTBOT_EMAIL is not configured',
				{ stream: 'system', domains: domainNames }
			);
			}
		} catch (error) {
		await recordDeploymentLog(deployment.id, 'error', 'Failed to configure TLS', {
			stream: 'system',
			error: error.message
		});
			throw error;
		}
	}

	const versionId = await recordDeploymentVersion({
		deploymentId: deployment.id,
		slot,
		commitSha: options.commitSha,
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

	await cleanupOldBuilds(deployment);

	return { slot, port, versionId };
}

export async function cleanupOldBuilds(deployment) {
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
		await logger.info('Cleaned up builds', { removed: toRemove });
	}
}

export async function activateVersion(deployment, version) {
	const domains = await listDomains(deployment.id);
	await sql`
    UPDATE deployment_versions
    SET status = CASE WHEN id = ${version.id} THEN 'active' ELSE 'inactive' END
    WHERE deployment_id = ${deployment.id}
  `;
	await configureDeploymentIngress({
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
}

export async function cleanupDeploymentResources(deployment) {
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
}

function ensureControllableSlot(deployment) {
	if (!deployment.active_slot) {
		throw new Error('Deploy this app at least once before using start/stop controls.');
	}
	return deployment.active_slot;
}

export async function stopDeploymentRuntime(deployment) {
	const slot = ensureControllableSlot(deployment);
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const config = getConfig();
	try {
		if (config.localMode) {
			await stopLocalService(serviceName);
		} else {
			await stopService(`${serviceName}.service`);
		}
	} catch (error) {
		await recordDeploymentLog(
			deployment.id,
			'error',
			`Failed to stop deployment runtime: ${error.message}`,
			{
				stream: 'system',
				slot,
				service: serviceName
			}
		);
		throw error;
	}
	await updateDeployment(deployment.id, { status: 'inactive' });
	await recordDeploymentLog(deployment.id, 'info', 'Deployment stopped', {
		stream: 'system',
		slot,
		service: serviceName
	});
}

export async function startDeploymentRuntime(deployment) {
	const slot = ensureControllableSlot(deployment);
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const config = getConfig();
	if (config.localMode) {
		throw new Error('Start command is unavailable in local mode. Redeploy instead.');
	}
	try {
		await startService(`${serviceName}.service`);
	} catch (error) {
		await recordDeploymentLog(
			deployment.id,
			'error',
			`Failed to start deployment runtime: ${error.message}`,
			{
				stream: 'system',
				slot,
				service: serviceName
			}
		);
		throw error;
	}
	await updateDeployment(deployment.id, { status: 'running' });
	await recordDeploymentLog(deployment.id, 'info', 'Deployment resumed', {
		stream: 'system',
		slot,
		service: serviceName
	});
}
