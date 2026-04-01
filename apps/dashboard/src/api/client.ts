import type {
  DashboardResponse,
  Environment,
  RunPlanResponse,
  RunsResponse,
  SchemaDriftResponse,
  SeedApplyResponse,
  SeedInventoryEntry,
  SeedPlanResponse,
  SeedsResponse,
} from "../types";
import { getAPIKey } from "../auth";

export class APIError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

async function requestJSON<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> {
  const apiKey = getAPIKey();
  const res = await fetch(input, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}),
      ...(init?.headers ?? {}),
    },
  });
  if (!res.ok) {
    const raw = await res.text().catch(() => "");
    // Our Go server returns JSON like {"error":"..."} for many failures.
    // Prefer showing that message instead of the raw JSON string.
    const parsedError = (() => {
      try {
        const parsed = raw ? (JSON.parse(raw) as { error?: unknown }) : null;
        if (parsed && typeof parsed.error === "string" && parsed.error.trim() !== "") {
          return parsed.error;
        }
      } catch {
        // ignore parse errors
      }
      return "";
    })();
    throw new APIError(parsedError || raw || `Request failed: ${res.status} ${res.statusText}`, res.status);
  }
  return (await res.json()) as T;
}

export async function getEnvironments(): Promise<{
  environments: Environment[];
}> {
  return requestJSON("/api/v1/environments");
}

export async function getDashboard(): Promise<DashboardResponse> {
  return requestJSON("/api/v1/dashboard");
}

export async function getRuns(limit = 20): Promise<RunsResponse> {
  const q = new URLSearchParams({ limit: String(limit) });
  return requestJSON(`/api/v1/runs?${q.toString()}`);
}

export async function getSeedRuns(limit = 20): Promise<RunsResponse> {
  const q = new URLSearchParams({ limit: String(limit) });
  return requestJSON(`/api/v1/seed-runs?${q.toString()}`);
}

export async function runEnvironment(envName: string): Promise<unknown> {
  const path = `/api/v1/environments/${encodeURIComponent(envName)}/run`;
  return requestJSON(path, { method: "POST" });
}

export async function planEnvironment(envName: string): Promise<RunPlanResponse> {
  const path = `/api/v1/environments/${encodeURIComponent(envName)}/run`;
  return requestJSON(path, {
    method: "POST",
    body: JSON.stringify({ mode: "plan" }),
  });
}

export async function applyEnvironment(envName: string, planId?: string): Promise<unknown> {
  const path = `/api/v1/environments/${encodeURIComponent(envName)}/run`;
  return requestJSON(path, {
    method: "POST",
    body: JSON.stringify({ mode: "apply", plan_id: planId }),
  });
}

export async function getSchemaDrift(envName: string): Promise<SchemaDriftResponse> {
  const q = new URLSearchParams({ env: envName });
  return requestJSON(`/api/v1/drift/schema?${q.toString()}`);
}

export async function getSeeds(): Promise<SeedsResponse> {
  return requestJSON("/api/v1/seeds");
}

export async function getSeedById(seedId: string): Promise<SeedInventoryEntry> {
  return requestJSON(`/api/v1/seeds/${encodeURIComponent(seedId)}`);
}

export async function planSeeds(envName: string): Promise<SeedPlanResponse> {
  return requestJSON(`/api/v1/environments/${encodeURIComponent(envName)}/seeds/plan`, {
    method: "POST",
  });
}

export async function applySeeds(envName: string, planId?: string): Promise<SeedApplyResponse> {
  return requestJSON(`/api/v1/environments/${encodeURIComponent(envName)}/seeds/apply`, {
    method: "POST",
    body: JSON.stringify({ plan_id: planId }),
  });
}

