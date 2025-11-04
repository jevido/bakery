import { Router } from './router.js';
import { AuthController } from '../controllers/authController.js';
import { DeploymentController } from '../controllers/deploymentController.js';
import { DomainController } from '../controllers/domainController.js';
import { DatabaseController } from '../controllers/databaseController.js';
import { SystemController } from '../controllers/systemController.js';
import { GithubController } from '../controllers/githubController.js';

export const router = new Router();

router.post('/api/auth/register', (ctx) => AuthController.register(ctx));
router.post('/api/auth/login', (ctx) => AuthController.login(ctx));
router.post('/api/auth/logout', (ctx) => AuthController.logout(ctx), { auth: true });
router.get('/api/auth/me', (ctx) => AuthController.me(ctx), { auth: true });
router.get('/api/auth/github/url', (ctx) => AuthController.githubStart(ctx), { auth: true });
router.get('/api/auth/github/callback', (ctx) => AuthController.githubCallback(ctx));

router.get('/api/deployments', (ctx) => DeploymentController.list(ctx), { auth: true });
router.post('/api/deployments', (ctx) => DeploymentController.create(ctx), { auth: true });
router.get('/api/deployments/:id', (ctx) => DeploymentController.detail(ctx), { auth: true });
router.post('/api/deployments/:id/deploy', (ctx) => DeploymentController.triggerDeploy(ctx), {
  auth: true
});
router.post('/api/deployments/:id/restart', (ctx) => DeploymentController.restart(ctx), {
  auth: true
});
router.post('/api/deployments/:id/rollback', (ctx) => DeploymentController.rollback(ctx), {
  auth: true
});
router.post('/api/deployments/:id/env', (ctx) => DeploymentController.updateEnv(ctx), {
  auth: true
});
router.delete('/api/deployments/:id/env', (ctx) => DeploymentController.deleteEnv(ctx), {
  auth: true
});
router.post('/api/deployments/:id/domains', (ctx) => DeploymentController.addDomain(ctx), {
  auth: true
});
router.delete('/api/deployments/:id/domains', (ctx) => DeploymentController.removeDomain(ctx), {
  auth: true
});
router.get(
  '/api/deployments/:id/logs/stream',
  (ctx) => DeploymentController.streamLogs(ctx),
  { auth: true }
);
router.post('/api/deployments/:id/databases', (ctx) => DatabaseController.create(ctx), {
  auth: true
});
router.get('/api/deployments/:id/databases', (ctx) => DatabaseController.list(ctx), {
  auth: true
});
router.delete('/api/deployments/:id/databases', (ctx) => DatabaseController.remove(ctx), {
  auth: true
});

router.post('/api/domains/:id/verify', (ctx) => DomainController.verify(ctx), { auth: true });

router.get('/api/system/health', (ctx) => SystemController.health(ctx));
router.get('/api/system/analytics', (ctx) => SystemController.analytics(ctx), { auth: true });
router.post('/api/system/update', (ctx) => SystemController.update(ctx), { auth: true });

router.get('/api/github/repos', (ctx) => GithubController.repositories(ctx), { auth: true });
router.get('/api/github/branches', (ctx) => GithubController.branches(ctx), { auth: true });
