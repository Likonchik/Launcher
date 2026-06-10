// Общие хелперы авторизации дашборда: единый ключ токена, базовый URL API и
// функция запроса. Используются страницей /login и guard'ом (AppFrame).

export const apiUrl = (process.env.NEXT_PUBLIC_API_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');
export const tokenKey = 'launcher.admin.token';

export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(tokenKey);
}

export function setToken(token: string): void {
  localStorage.setItem(tokenKey, token);
}

export function clearToken(): void {
  localStorage.removeItem(tokenKey);
}

export type AuthUser = { id: string; login: string; role: string };

export async function api<T = unknown>(
  path: string,
  options: { method?: string; token?: string | null; body?: unknown } = {}
): Promise<T> {
  const response = await fetch(`${apiUrl}${path}`, {
    method: options.method ?? 'GET',
    headers: {
      Accept: 'application/json',
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.token ? { Authorization: `Bearer ${options.token}` } : {})
    },
    body: options.body ? JSON.stringify(options.body) : undefined
  });
  if (response.status === 204) {
    return undefined as T;
  }
  const data = (await response.json().catch(() => ({}))) as { message?: string } & T;
  if (!response.ok) {
    throw new Error(data.message ?? `HTTP ${response.status}`);
  }
  return data;
}
