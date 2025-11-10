#!/usr/bin/env bun
import { access, chmod, copyFile, mkdir, readFile, readdir, realpath, writeFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const APP_DIR = path.resolve(__dirname, '../..');

async function pathExists(target) {
  try {
    await access(target, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

async function runCommand(command, args = [], options = {}) {
  const { cwd = APP_DIR, env, check = true, capture = false } = options;
  const child = Bun.spawn([command, ...args], {
    cwd,
    env: env ? { ...process.env, ...env } : process.env,
    stdin: 'inherit',
    stdout: capture ? 'pipe' : 'inherit',
    stderr: capture ? 'pipe' : 'inherit'
  });

  let stdout = '';
  let stderr = '';
  if (capture && child.stdout) {
    stdout = await new Response(child.stdout).text();
  }
  if (capture && child.stderr) {
    stderr = await new Response(child.stderr).text();
  }

  const exitCode = await child.exited;
  if (check && exitCode !== 0) {
    const details = stderr || stdout;
    throw new Error(
      `Command ${command} ${args.join(' ')} failed (exit ${exitCode})${details ? `: ${details.trim()}` : ''}`
    );
  }

  return { exitCode, stdout, stderr };
}

async function updateRepository() {
  if (!(await pathExists(path.join(APP_DIR, '.git')))) {
    return;
  }
  await runCommand('git', ['config', '--global', '--add', 'safe.directory', APP_DIR], { check: false });
  await runCommand('git', ['pull', '--rebase']);
}

async function loadEnvFile() {
  const envPath = path.join(APP_DIR, '.env');
  if (!(await pathExists(envPath))) {
    return;
  }
  const content = await readFile(envPath, 'utf8');
  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    const match = line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$/);
    if (!match) continue;
    const [, key, valueRaw] = match;
    let value = valueRaw;
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    process.env[key] = value;
  }
}

function configureBunEnv() {
  const BUN_INSTALL_ROOT = '/usr/local/lib/bun';
  process.env.BUN_INSTALL = process.env.BUN_INSTALL || BUN_INSTALL_ROOT;
  const bunBin = path.join(process.env.BUN_INSTALL, 'bin');
  process.env.PATH = ['/usr/local/bin', bunBin, process.env.PATH || ''].filter(Boolean).join(':');
}

async function ensureBunBinary() {
  const target = '/usr/local/bin/bun';
  try {
    const resolved = await realpath(target);
    if (resolved.startsWith('/root/')) {
      await copyFile(resolved, target);
      await chmod(target, 0o755);
      return;
    }
    return;
  } catch {
    // fall through if the file doesn't exist yet
  }

  const candidate = path.join(process.env.BUN_INSTALL || '/usr/local/lib/bun', 'bin', 'bun');
  if (await pathExists(candidate)) {
    await copyFile(candidate, target);
    await chmod(target, 0o755);
  }
}

async function renderTemplate(from, to, replacements) {
  const template = await readFile(from, 'utf8');
  const rendered = template.replace(/\{\{([A-Za-z0-9_]+)\}\}/g, (_, key) => {
    if (!(key in replacements)) {
      throw new Error(`Missing template variable ${key}`);
    }
    return replacements[key];
  });
  await writeFile(to, rendered, 'utf8');
}

function isIpAddress(value) {
  if (!value) return false;
  if (/^[0-9.]+$/.test(value)) return true;
  return value.includes(':');
}

async function locateCertbotTlsFile(filename) {
  const roots = ['/usr/lib', '/usr/local/lib'];
  for (const root of roots) {
    const entries = await readdir(root, { withFileTypes: true }).catch(() => []);
    for (const entry of entries) {
      if (!entry.isDirectory() || !entry.name.startsWith('python')) continue;
      for (const pkgDir of ['dist-packages', 'site-packages']) {
        const candidate = path.join(root, entry.name, pkgDir, 'certbot_nginx', '_internal', 'tls_configs', filename);
        if (await pathExists(candidate)) {
          return candidate;
        }
      }
    }
  }
  return null;
}

async function ensureTlsDefaults() {
  const optionTarget = '/etc/letsencrypt/options-ssl-nginx.conf';
  const dhTarget = '/etc/letsencrypt/ssl-dhparams.pem';
  await mkdir('/etc/letsencrypt', { recursive: true });

  if (!(await pathExists(optionTarget))) {
    const src = await locateCertbotTlsFile('options-ssl-nginx.conf');
    if (src) {
      await copyFile(src, optionTarget);
    }
  }

  if (!(await pathExists(dhTarget))) {
    const src = await locateCertbotTlsFile('ssl-dhparams.pem');
    if (src) {
      await copyFile(src, dhTarget);
    }
  }
}

async function ensureControlPlaneCertificate(host, email) {
  if (!host) return false;
  const certDir = path.join('/etc/letsencrypt/live', host);
  const certExists =
    (await pathExists(path.join(certDir, 'fullchain.pem'))) &&
    (await pathExists(path.join(certDir, 'privkey.pem')));
  if (certExists) {
    console.log(`[nginx] Existing certificate detected for ${host}`);
    return true;
  }

  if (!email) {
    throw new Error(`Cannot request a Let's Encrypt certificate for ${host} without CERTBOT_EMAIL`);
  }

  console.log(`[nginx] Requesting Let's Encrypt certificate for ${host} (standalone)`);
  await runCommand('systemctl', ['stop', 'nginx'], { check: false });
  const result = await runCommand(
    'certbot',
    [
      'certonly',
      '--standalone',
      '--preferred-challenges',
      'http',
      '--agree-tos',
      '--non-interactive',
      '-m',
      email,
      '-d',
      host
    ],
    { check: false }
  );
  await runCommand('systemctl', ['start', 'nginx'], { check: false });

  if (result.exitCode === 0) {
    console.log(`[nginx] Certificate issued for ${host}`);
    await ensureTlsDefaults().catch(() => {});
    return true;
  }

  console.warn(`Warning: Certbot failed for ${host}; continuing without HTTPS.`);
  return false;
}

async function renderControlPlaneNginx(host, port, certbotEmail) {
  const templatePath = path.join(APP_DIR, 'infrastructure', 'nginx', 'templates', 'app.conf');
  const targetPath = '/etc/nginx/conf.d/bakery.conf';
  const logsDir = '/var/log/bakery';
  await mkdir(logsDir, { recursive: true });

  const upstreamName = 'bakery_control_plane';
  let httpsDomains = '';
  let httpRedirects = '';
  let primaryDomain = host || '_';
  let listenDirective = 'listen 80;';
  let http2Directive = '# http/1.1 only';
  let sslDirectives = '';
  const wantsHttps = !!host && !isIpAddress(host);
  let certReady = false;

  if (wantsHttps) {
    httpRedirects = `server {\n  listen 80;\n  server_name ${host};\n  return 301 https://${host}$request_uri;\n}\n`;
    certReady = await ensureControlPlaneCertificate(host, certbotEmail);
  }

  if (wantsHttps && certReady) {
    httpsDomains = `server_name ${host};`;
    primaryDomain = host;
    listenDirective = 'listen 443 ssl;';
    http2Directive = 'http2 on;';
    const certBase = `/etc/letsencrypt/live/${host}`;
    const directives = [
      `    ssl_certificate ${certBase}/fullchain.pem;`,
      `    ssl_certificate_key ${certBase}/privkey.pem;`
    ];
    if (await pathExists('/etc/letsencrypt/options-ssl-nginx.conf')) {
      directives.push('    include /etc/letsencrypt/options-ssl-nginx.conf;');
    }
    if (await pathExists('/etc/letsencrypt/ssl-dhparams.pem')) {
      directives.push('    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;');
    }
    sslDirectives = directives.join('\n');
  } else {
    httpsDomains = `server_name ${host ?? '_'};`;
    httpRedirects = '';
  }

  await renderTemplate(templatePath, targetPath, {
    UPSTREAM_NAME: upstreamName,
    PORT: String(port),
    HTTPS_DOMAINS: httpsDomains,
    HTTP_REDIRECT_BLOCKS: httpRedirects,
    LISTEN_DIRECTIVE: listenDirective,
    HTTP2_DIRECTIVE: http2Directive,
    SSL_DIRECTIVES: sslDirectives,
    ACCESS_LOG: path.join(logsDir, 'control-plane-access.log'),
    ERROR_LOG: path.join(logsDir, 'control-plane-error.log'),
    PRIMARY_DOMAIN: primaryDomain
  });

  if (wantsHttps) {
    await ensureControlPlaneCertificate(host, certbotEmail);
  }
}

async function renderSystemdService() {
  const templatePath = path.join(APP_DIR, 'infrastructure', 'systemd', 'bakery.service');
  const targetPath = '/etc/systemd/system/bakery.service';
  await renderTemplate(templatePath, targetPath, { WORKING_DIR: APP_DIR });
}

async function renderUpdateTimer() {
  const serviceTemplate = path.join(APP_DIR, 'infrastructure', 'systemd', 'bakery-update.service');
  const timerTemplate = path.join(APP_DIR, 'infrastructure', 'systemd', 'bakery-update.timer');
  await renderTemplate(serviceTemplate, '/etc/systemd/system/bakery-update.service', { WORKING_DIR: APP_DIR });
  await copyFile(timerTemplate, '/etc/systemd/system/bakery-update.timer');
}

async function chmodSafe(target, mode) {
  if (await pathExists(target)) {
    await chmod(target, mode);
  }
}

function extractHostname(value) {
  if (!value) return '';
  try {
    const url = new URL(value);
    return url.hostname;
  } catch {
    return value.replace(/^https?:\/\//, '').split(/[/:]/)[0];
  }
}

async function reloadNginx() {
  const test = await runCommand('nginx', ['-t'], { check: false });
  if (test.exitCode !== 0) {
    console.warn('Warning: nginx configuration test failed. Review /etc/nginx/conf.d/bakery.conf');
    return;
  }

  const active = await runCommand('systemctl', ['is-active', '--quiet', 'nginx'], { check: false });
  if (active.exitCode === 0) {
    const reloaded = await runCommand('systemctl', ['reload', 'nginx'], { check: false });
    if (reloaded.exitCode !== 0) {
      const restarted = await runCommand('systemctl', ['restart', 'nginx'], { check: false });
      if (restarted.exitCode !== 0) {
        await runCommand('systemctl', ['start', 'nginx'], { check: false });
      }
    }
  } else {
    await runCommand('systemctl', ['start', 'nginx'], { check: false });
  }
}

async function main() {
  console.log(`Updating Bakery in ${APP_DIR}`);
  process.chdir(APP_DIR);

  await updateRepository();
  await loadEnvFile();
  configureBunEnv();
  await ensureBunBinary();

  await runCommand('bun', ['--bun', 'install']);
  await runCommand('bun', ['--bun', 'install'], { cwd: path.join(APP_DIR, 'app') });
  await runCommand('bun', ['run', 'build'], { cwd: path.join(APP_DIR, 'app') });
  await chmodSafe(path.join(APP_DIR, 'app', 'build', 'index.js'), 0o755);

  await runCommand('bun', ['run', 'migrate']);
  await renderSystemdService();
  await renderUpdateTimer();

  const controlPlanePort = Number(process.env.BAKERY_PORT || 4100);
  const controlPlaneHost = extractHostname(process.env.BAKERY_BASE_URL || '');
  await renderControlPlaneNginx(controlPlaneHost, controlPlanePort, process.env.CERTBOT_EMAIL || '');
  await reloadNginx();

  await runCommand('systemctl', ['daemon-reload'], { check: false });
  await runCommand('systemctl', ['restart', 'bakery-update.timer'], { check: false });
  await runCommand('systemctl', ['restart', 'bakery.service']);

  console.log('Bakery updated and restarted.');
}

await main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});
