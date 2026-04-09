const API_BASE = (process.env.NEXT_PUBLIC_VELARIX_API_URL || "http://localhost:8080").replace(/\/$/, "");

export function apiUrl(path: string): string {
  if (!path.startsWith("/")) {
    path = `/${path}`;
  }
  return `${API_BASE}${path}`;
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers || {});
  return fetch(apiUrl(path), {
    ...init,
    headers,
    credentials: "include",
  });
}
