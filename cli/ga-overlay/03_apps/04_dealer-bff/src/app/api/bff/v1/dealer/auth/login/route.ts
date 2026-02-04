/**
 * POST /api/bff/v1/dealer/auth/login
 * Dealer login endpoint - API-P-00-02
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

// Demo dealer users (in production, use DB)
const DEMO_DEALER_USERS = [
  {
    id: 'dealer-user-1',
    dealerId: 'dlr-1',
    email: 'user@dlr001.com',
    passwordHash: '8d23cf6c86e834a7aa6eded54c26ce2bb2e74903538c61bdd5d2197997ab2f72', // dealer123
    claims: ['dealer:profile', 'dealer:orders'],
  },
];

function hashPassword(password: string): string {
  const crypto = require('crypto');
  return crypto.createHash('sha256').update(password).digest('hex');
}

export const POST = withPublicEnvelope('REF-P-00', 'API-P-00-02', async (req, { envelope }) => {
  logger.info('Dealer login attempt', { api: envelope.api });

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
    const user = DEMO_DEALER_USERS.find(
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

  // Generate tokens with Dealer namespace
  const pid = generatePid('D', user.id);
  const tokens = await mintTokenPair({
    userId: user.id,
    namespace: 'D',
    pid,
    claims: user.claims,
    dealerId: user.dealerId,
    family: generateTokenFamily(),
  });

  const atCookie = createAccessTokenCookie(tokens.accessToken);
  const rtCookie = createRefreshTokenCookie(tokens.refreshToken);

  logger.info('Dealer login successful', { userId: user.id, dealerId: user.dealerId, pid });

  return new Response(
    JSON.stringify({
      success: true,
      user: { id: user.id, email: user.email, dealerId: user.dealerId },
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
