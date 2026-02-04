/**
 * POST /api/bff/v1/admin/auth/refresh
 * Refresh access token endpoint - API-P-00-01
 *
 * Rate Limit: auth/refresh (20 req/min per PID)
 * Idempotency: Not required (refresh is naturally idempotent - generates new tokens)
 */

import { NextRequest } from 'next/server';
import {
  withPublicEnvelope,
  withService,
  jsonResponse,
  isServiceSuccess,
} from '@b2b/application';
import {
  verifyRefreshToken,
  extractRefreshToken,
  mintAccessToken,
  mintRefreshToken,
  createAccessTokenCookie,
  createRefreshTokenCookie,
  formatCookieHeader,
} from '@b2b/auth';
import { logger } from '@b2b/observability';
import { rateLimit, isRateLimited } from '@b2b/rate-limit';

// Demo user claims (in production, fetch from DB)
const DEMO_CLAIMS: Record<string, string[]> = {
  'user-admin-1': ['admin:superuser', 'admin:dealer-management'],
};

export const POST = withPublicEnvelope('REF-P-00', 'API-P-00-01', async (req, { envelope }) => {
  logger.info('Token refresh attempt', { api: envelope.api });

  // Rate limit check (auth/refresh policy: 20 req/min per PID+API)
  const ip = req.headers.get('x-forwarded-for')?.split(',')[0] || 'unknown';
  const rateLimitResult = await rateLimit({ apiId: envelope.api, ip }, 'auth/refresh');
  if (isRateLimited(rateLimitResult)) {
    return jsonResponse(
      { error: 'Rate limited', code: 'RATE_LIMITED' },
      429,
      { 'Retry-After': String(rateLimitResult.resetInSec) }
    );
  }

  const cookieHeader = req.headers.get('cookie');
  const refreshToken = extractRefreshToken(cookieHeader);

  if (!refreshToken) {
    return jsonResponse({ error: 'No refresh token provided', code: 'NO_TOKEN' }, 401);
  }

  // Verify refresh token
  const verifyResult = await verifyRefreshToken(refreshToken);

  if (!verifyResult.valid) {
    logger.warn('Invalid refresh token', { error: verifyResult.error });
    return jsonResponse({ error: 'Invalid refresh token', code: verifyResult.error }, 401);
  }

  const { payload } = verifyResult;

  // TODO: Validate token against database (check not revoked, check family for reuse detection)

  // Generate new tokens
  const result = await withService('SVC-P-00-02', async () => {
    const claims = DEMO_CLAIMS[payload.sub] || [];

    const [at, rt] = await Promise.all([
      mintAccessToken({
        userId: payload.sub,
        namespace: payload.namespace,
        pid: payload.pid,
        claims,
      }),
      mintRefreshToken({
        userId: payload.sub,
        namespace: payload.namespace,
        pid: payload.pid,
        family: payload.family,
      }),
    ]);

    return { at, rt };
  });

  if (!isServiceSuccess(result)) {
    return jsonResponse({ error: 'Token refresh failed', code: 'REFRESH_FAILED' }, 500);
  }

  const { at, rt } = result.data;

  // Create cookie headers
  const atCookie = createAccessTokenCookie(at.token);
  const rtCookie = createRefreshTokenCookie(rt.token);

  logger.info('Token refresh successful', { pid: payload.pid });

  // TODO: Rotate refresh token in database

  return new Response(
    JSON.stringify({
      success: true,
      expiresAt: at.expiresAt,
    }),
    {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'Set-Cookie': [formatCookieHeader(atCookie), formatCookieHeader(rtCookie)].join(', '),
      },
    }
  );
});
