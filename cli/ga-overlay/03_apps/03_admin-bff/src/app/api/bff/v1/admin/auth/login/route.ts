/**
 * POST /api/bff/v1/admin/auth/login
 * Admin login endpoint - API-P-00-02
 *
 * Rate Limit: auth/login (10 req/min per IP)
 * Idempotency: Not required (login is naturally idempotent by session)
 */

import { NextRequest } from 'next/server';
import {
  withPublicEnvelope,
  withService,
  jsonResponse,
  isServiceSuccess,
} from '@b2b/application';
import {
  mintTokenPair,
  createAccessTokenCookie,
  createRefreshTokenCookie,
  formatCookieHeader,
  generateTokenFamily,
} from '@b2b/auth';
import { generatePid } from '@b2b/identity';
import { logger } from '@b2b/observability';
import { rateLimit, isRateLimited } from '@b2b/rate-limit';

// For demo purposes - in production, use proper password hashing and DB lookup
const DEMO_USERS = [
  {
    id: 'user-admin-1',
    email: 'admin@b2b-portal.com',
    passwordHash: '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9', // admin123
    claims: ['admin:superuser', 'admin:dealer-management'],
  },
];

function hashPassword(password: string): string {
  const crypto = require('crypto');
  return crypto.createHash('sha256').update(password).digest('hex');
}

export const POST = withPublicEnvelope('REF-P-00', 'API-P-00-02', async (req, { envelope }) => {
  logger.info('Login attempt', { api: envelope.api });

  // Rate limit check (auth/login policy: 10 req/min per IP)
  const ip = req.headers.get('x-forwarded-for')?.split(',')[0] || 'unknown';
  const rateLimitResult = await rateLimit({ apiId: envelope.api, ip }, 'auth/login');
  if (isRateLimited(rateLimitResult)) {
    return jsonResponse(
      { error: 'Too many login attempts', code: 'RATE_LIMITED' },
      429,
      {
        'Retry-After': String(rateLimitResult.resetInSec),
        'X-RateLimit-Limit': String(rateLimitResult.limit),
        'X-RateLimit-Remaining': '0',
      }
    );
  }

  const body = await req.json();
  const { email, password } = body as { email: string; password: string };

  if (!email || !password) {
    return jsonResponse({ error: 'Email and password required', code: 'VALIDATION_ERROR' }, 400);
  }

  const result = await withService('SVC-P-00-01', async () => {
    const user = DEMO_USERS.find(
      (u) => u.email === email && u.passwordHash === hashPassword(password)
    );

    if (!user) {
      throw new Error('Invalid credentials');
    }

    return user;
  });

  if (!isServiceSuccess(result)) {
    return jsonResponse({ error: 'Invalid credentials', code: 'INVALID_CREDENTIALS' }, 401);
  }

  const user = result.data;

  // Generate tokens
  const pid = generatePid('A', user.id);
  const tokens = await mintTokenPair({
    userId: user.id,
    namespace: 'A',
    pid,
    claims: user.claims,
    family: generateTokenFamily(),
  });

  // Create cookie headers
  const atCookie = createAccessTokenCookie(tokens.accessToken);
  const rtCookie = createRefreshTokenCookie(tokens.refreshToken);

  logger.info('Login successful', { userId: user.id, pid });

  // TODO: Store refresh token in database

  return new Response(
    JSON.stringify({
      success: true,
      user: { id: user.id, email: user.email },
      expiresAt: tokens.accessTokenExpiresAt,
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
