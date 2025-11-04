import { authenticateRequest } from '../lib/auth.js';
import { getConfig } from '../lib/config.js';

function compilePath(path) {
  const keys = [];
  const pattern = path
    .split('/')
    .map((segment) => {
      if (segment.startsWith(':')) {
        keys.push(segment.slice(1));
        return '([^/]+)';
      }
      return segment;
    })
    .join('/');
  const regex = new RegExp(`^${pattern}$`);
  return { regex, keys };
}

async function parseBody(request) {
  const type = request.headers.get('content-type') || '';
  if (type.includes('application/json')) {
    return request.json();
  }
  if (type.includes('application/x-www-form-urlencoded')) {
    const form = await request.formData();
    return Object.fromEntries(form.entries());
  }
  if (type.includes('text/plain')) {
    return request.text();
  }
  return null;
}

export class Router {
  constructor() {
    this.routes = [];
    this.middlewares = [];
  }

  use(middleware) {
    this.middlewares.push(middleware);
  }

  register(method, path, handler, options = {}) {
    const { regex, keys } = compilePath(path);
    this.routes.push({
      method: method.toUpperCase(),
      path,
      regex,
      keys,
      handler,
      options
    });
  }

  options(path, handler) {
    this.register('OPTIONS', path, handler);
  }

  get(path, handler, options) {
    this.register('GET', path, handler, options);
  }

  post(path, handler, options) {
    this.register('POST', path, handler, options);
  }

  put(path, handler, options) {
    this.register('PUT', path, handler, options);
  }

  patch(path, handler, options) {
    this.register('PATCH', path, handler, options);
  }

  delete(path, handler, options) {
    this.register('DELETE', path, handler, options);
  }

  async handle(request) {
    const url = new URL(request.url);
    const config = getConfig();
    const method = request.method.toUpperCase();

    if (method === 'OPTIONS') {
      return new Response(null, {
        status: 204,
        headers: this.corsHeaders(request, config)
      });
    }

    const user = await authenticateRequest(request);

    const route = this.routes.find((candidate) => {
      if (candidate.method !== method) return false;
      return candidate.regex.test(url.pathname);
    });

    if (!route) {
      return new Response(
        JSON.stringify({ error: 'Not Found' }),
        {
          status: 404,
          headers: {
            'content-type': 'application/json',
            ...this.corsHeaders(request, config)
          }
        }
      );
    }

    const matches = url.pathname.match(route.regex);
    const params = {};
    if (matches) {
      route.keys.forEach((key, idx) => {
        params[key] = decodeURIComponent(matches[idx + 1]);
      });
    }

    const ctx = {
      request,
      url,
      params,
      user,
      config,
      get body() {
        if (!this._bodyPromise) {
          this._bodyPromise = parseBody(request);
        }
        return this._bodyPromise;
      },
      async json(data, status = 200, headers = {}) {
        return new Response(JSON.stringify(data), {
          status,
          headers: {
            'content-type': 'application/json',
            ...this.corsHeaders(request, config),
            ...headers
          }
        });
      },
      async text(data, status = 200, headers = {}) {
        return new Response(data, {
          status,
          headers: {
            'content-type': 'text/plain',
            ...this.corsHeaders(request, config),
            ...headers
          }
        });
      },
      corsHeaders: this.corsHeaders.bind(this)
    };

    try {
      for (const middleware of this.middlewares) {
        const result = await middleware(ctx);
        if (result instanceof Response) {
          return result;
        }
      }

      if (route.options?.auth && !user) {
        return ctx.json({ error: 'Unauthorized' }, 401);
      }

      const response = await route.handler(ctx);
      if (!response) {
        return ctx.json({ ok: true });
      }
      return response;
    } catch (error) {
      console.error(error);
      return ctx.json({ error: 'Internal Server Error', details: error.message }, 500);
    }
  }

  corsHeaders(request, config) {
    const origin = request.headers.get('origin');
    const allowed =
      config.allowedOrigins.length === 0 ||
      (origin && config.allowedOrigins.includes(origin));
    return {
      'access-control-allow-origin': allowed && origin ? origin : '*',
      'access-control-allow-headers':
        'Content-Type, Authorization, X-Requested-With',
      'access-control-allow-methods': 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
      'access-control-allow-credentials': 'true'
    };
  }
}
