import { BASE_URL } from './base-url.js';

// loginAdmin signs in through the web login (POST /api/auth/login) with the
// demo admin credentials from .env, stores the access_token cookie in the
// page context and returns the token for request contexts that need a
// Cookie header of their own.
export async function loginAdmin(page) {
  const username = process.env.DEMO_ADMIN_EMAIL || 'root@localhost';
  const password = process.env.DEMO_ADMIN_PASSWORD;
  if (!password) {
    throw new Error('DEMO_ADMIN_PASSWORD must be set in .env');
  }
  const response = await page.request.post(`${BASE_URL}/api/auth/login`, {
    data: { username, password },
    headers: { 'Content-Type': 'application/json' },
  });
  if (!response.ok()) {
    throw new Error(`login failed for ${username}: ${response.status()} ${response.statusText()}`);
  }
  const payload = await response.json();
  const token = payload && payload.access_token;
  if (!token) {
    throw new Error('login response missing access_token');
  }
  await page.context().addCookies([
    {
      name: 'access_token',
      value: token,
      url: `${BASE_URL}/`,
      httpOnly: false,
      secure: false,
      sameSite: 'Lax',
    },
  ]);
  return token;
}
