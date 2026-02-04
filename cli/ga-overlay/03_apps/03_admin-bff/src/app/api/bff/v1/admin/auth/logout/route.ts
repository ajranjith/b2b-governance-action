/**
 * POST /api/bff/v1/admin/auth/logout
 * Logout endpoint - API-P-00-03
 *
 * Rate Limit: auth/refresh (20 req/min per PID) - use refresh policy as logout is similar
 * Idempotency: Naturally idempotent (logout always succeeds)
 */

import { NextRequest } from 'next/server';
import {
  withPublicEnvelope,
  withService,
  jsonResponse,
  isServiceSuccess,
} from '@b2b/application';
import {
  createLogoutCookies,
  formatCookieHeader,
  extractRefreshToken,
  verifyRefreshToken,
} from '@b2b/auth';
import { logger } from '@b2b/observability';
import { rateLimit, isRateLimited } from '@b2b/rate-limit';

export const POST = withPublicEnvelope('REF-P-00', 'API-P-00-03', async (req, { envelope }) => {
  logger.info('Logout attempt', { api: envelope.api });

  // Rate limit check (use auth/refresh policy for logout)
  const ip = req.headers.get('x-forwarded-for')?.split(',')[0] || 'unknown';
  const rateLimitResult = await rateLimit({ apiId: envelope.api, ip }, 'auth/refresh');
  if (isRateLimited(rateLimitResult)) {
    return jsonResponse({ error: 'Rate limited', code: 'RATE_LIMITED' }, 429);
  }

  const cookieHeader = req.headers.get('cookie');
  const refreshToken = extractRefreshToken(cookieHeader);

  // Revoke token if present
  if (refreshToken) {
    const verifyResult = await verifyRefreshToken(refreshToken);

    if (verifyResult.valid) {
      const result = await withService('SVC-P-00-03', async () => {
        // TODO: Revoke refresh token in database
        logger.info('Revoking refresh token', { pid: verifyResult.payload.pid });
      });

      if (!isServiceSuccess(result)) {
        return jsonResponse({ error: 'Logout failed', code: 'LOGOUT_FAILED' }, 500);
      }
    }
  }

  // Clear cookies
  const logoutCookies = createLogoutCookies();

  logger.info('Logout successful');

  return new Response(
    JSON.stringify({ success: true }),
    {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'Set-Cookie': logoutCookies.map(formatCookieHeader).join(', '),
      },
    }
  );
});
