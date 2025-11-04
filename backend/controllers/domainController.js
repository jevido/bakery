import { Resolver } from 'node:dns/promises';
import { findDomainById, updateDomain } from '../models/domainModel.js';
import { getConfig } from '../lib/config.js';

const resolver = new Resolver();

export const DomainController = {
  async verify(ctx) {
    const { id } = ctx.params;
    const domain = await findDomainById(id);
    if (!domain) {
      return ctx.json({ error: 'Domain not found' }, 404);
    }
    const config = getConfig();
    try {
      const records = await resolver.resolve4(domain.hostname);
      const verified = records.includes(config.publicIp);
      await updateDomain(domain.id, {
        verified,
        verification_checked_at: new Date(),
        ssl_status: verified ? 'pending' : 'unverified'
      });
      return ctx.json({ verified, records });
    } catch (error) {
      await updateDomain(domain.id, {
        verified: false,
        verification_checked_at: new Date(),
        ssl_status: 'unverified'
      });
      return ctx.json({ verified: false, error: error.message }, 500);
    }
  }
};
