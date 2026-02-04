/**
 * GET /api/bff/v1/admin/db-ping
 * Admin DB ping endpoint - API-A-00-03
 */

import { NextRequest } from 'next/server';
import {
  withAdminEnvelope,
  requirePolicy,
  withService,
  jsonResponse,
  isServiceSuccess,
} from '@b2b/application';
import { pingDb, logger } from '@b2b/database';

export const GET = withAdminEnvelope('REF-A-00', 'API-A-00-03', async (req, { envelope, token }) => {
  logger.info('Admin DB ping request', { api: envelope.api });

  requirePolicy('POL-A-99', token);

  const result = await withService('SVC-A-00-03', async () => {
    return pingDb();
  });

  if (!isServiceSuccess(result)) {
    return jsonResponse({ error: result.error.message, code: result.error.code }, result.error.statusCode);
  }

  return jsonResponse(result.data);
});
