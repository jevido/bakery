import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';

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

export async function writeDeploymentConfig({ deployment, domains, port, slot }) {
  const config = getConfig();
  const domainList = domains.map((d) => d.hostname).join(' ');
  const primaryDomain = domains[0] ? domains[0].hostname : `${deployment.id}.local`; 
  const httpsDomains = domains.map((d) => `server_name ${d.hostname};`).join('\n  ');
  const httpRedirects = domains
    .map(
      (d) => `
server {
  listen 80;
  server_name ${d.hostname};
  return 301 https://${d.hostname}$request_uri;
}
`
    )
    .join('\n');

  const upstreamName = `bakery_${deployment.id}_${slot}`;

  const nginxBody = await renderTemplate('app.conf', {
    UPSTREAM_NAME: upstreamName,
    PORT: port,
    HTTPS_DOMAINS: httpsDomains,
    HTTP_REDIRECT_BLOCKS: httpRedirects,
    ACCESS_LOG: join(config.logsDir, `${deployment.id}-${slot}-access.log`),
    ERROR_LOG: join(config.logsDir, `${deployment.id}-${slot}-error.log`),
    PRIMARY_DOMAIN: primaryDomain
  });

  await mkdir(config.nginxSitesDir, { recursive: true });
  const targetPath = join(config.nginxSitesDir, `${deployment.id}.conf`);
  await writeFile(targetPath, nginxBody, 'utf8');
  await log('info', 'Wrote nginx config', { targetPath, domainList });
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
