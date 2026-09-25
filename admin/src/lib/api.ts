// Client for the Missale API's /v1/admin routes. The session lives in
// localStorage (this page is used by one or two admins on their own
// computers); a 401 refreshes it once, then sends the admin back to /login.

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
export const CONTENT_URL = process.env.NEXT_PUBLIC_CONTENT_URL ?? "";

export type Field = { key: string; label: string; type: "text" | "longtext"; required: boolean; help?: string };
export type Collection = { key: string; label: string; description: string; fields: Field[] };
export type Item = {
  collection: string;
  lang: string;
  id: string;
  position: number;
  data: Record<string, string>;
  updatedAt: string;
  updatedBy: string;
};
export type Release = { version: number; publishedAt: string; publishedBy: string; items: number };
export type Session = {
  accessToken?: string;
  refreshToken?: string;
  expiresIn?: number;
  newPasswordNeeded?: boolean;
  session?: string;
};

export const LANGUAGES = [
  { key: "pt", label: "Português" },
  { key: "en", label: "English" },
  { key: "es", label: "Español" },
] as const;

const STORAGE_KEY = "missale-admin-session";

type Stored = { accessToken: string; refreshToken: string; email: string };

export function storedSession(): Stored | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Stored) : null;
  } catch {
    return null;
  }
}

export function saveSession(s: Stored) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
}

export function signOut() {
  localStorage.removeItem(STORAGE_KEY);
}

export class ApiError extends Error {
  constructor(public status: number, public code: string, public detail?: string) {
    super(detail ?? code);
  }
}

async function call<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers,
    },
  });
  if (res.status === 204) return undefined as T;
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, body.error ?? "error", body.detail);
  return body as T;
}

/** Calls an admin route, refreshing the session once on 401. */
async function authed<T>(path: string, init: RequestInit = {}): Promise<T> {
  const s = storedSession();
  if (!s) throw new ApiError(401, "unauthorized");
  try {
    return await call<T>(path, init, s.accessToken);
  } catch (e) {
    if (!(e instanceof ApiError) || e.status !== 401) throw e;
    try {
      const renewed = await call<Session>("/v1/admin/auth/refresh", {
        method: "POST",
        body: JSON.stringify({ refreshToken: s.refreshToken }),
      });
      saveSession({ ...s, accessToken: renewed.accessToken! });
      return await call<T>(path, init, renewed.accessToken);
    } catch {
      signOut();
      throw new ApiError(401, "session_expired");
    }
  }
}

export const api = {
  signIn: (email: string, password: string) =>
    call<Session>("/v1/admin/auth/signin", { method: "POST", body: JSON.stringify({ email, password }) }),
  newPassword: (email: string, session: string, newPassword: string) =>
    call<Session>("/v1/admin/auth/new-password", {
      method: "POST",
      body: JSON.stringify({ email, session, newPassword }),
    }),
  me: () => authed<{ email: string }>("/v1/admin/me"),
  collections: () => authed<{ collections: Collection[] }>("/v1/admin/collections"),
  list: (collection: string, lang: string) =>
    authed<{ items: Item[] }>(`/v1/admin/content/${collection}/${lang}`),
  save: (collection: string, lang: string, id: string, data: Record<string, string>, position?: number) =>
    authed<Item>(`/v1/admin/content/${collection}/${lang}/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(position === undefined ? { data } : { data, position }),
    }),
  remove: (collection: string, lang: string, id: string) =>
    authed<void>(`/v1/admin/content/${collection}/${lang}/${encodeURIComponent(id)}`, { method: "DELETE" }),
  publish: () => authed<Release>("/v1/admin/publish", { method: "POST" }),
  releases: () => authed<{ releases: Release[] }>("/v1/admin/releases"),
};

/** Portuguese message for an API error, for toasts and forms. */
export function describe(e: unknown): string {
  if (!(e instanceof ApiError)) return "Não foi possível falar com o servidor. Verifique a conexão.";
  switch (e.code) {
    case "invalid_credentials":
      return "E-mail ou senha incorretos.";
    case "forbidden":
      return "Esta conta não tem acesso ao painel.";
    case "session_expired":
    case "unauthorized":
      return "A sessão expirou. Entre de novo.";
    case "invalid_content":
      return `Conteúdo inválido: ${e.detail ?? ""}`;
    case "not_found":
      return "Item não encontrado.";
    default:
      return `Erro do servidor (${e.code}).`;
  }
}
