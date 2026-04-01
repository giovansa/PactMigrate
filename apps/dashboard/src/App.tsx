import * as React from "react";

import type {
  DashboardResponse,
  Environment,
  RunRecord,
  RunPlanResponse,
  SchemaDriftResponse,
  SeedInventoryEntry,
  SeedPlanResponse,
  SeedValidationIssue,
  SeedsResponse,
} from "./types";
import {
  APIError,
  applyEnvironment,
  applySeeds,
  getDashboard,
  getEnvironments,
  getRuns,
  getSeedRuns,
  getSeedById,
  getSchemaDrift,
  getSeeds,
  planEnvironment,
  planSeeds,
} from "./api/client";
import { clearAPIKey, getAPIKey, setAPIKey } from "./auth";
import { AppLayout, type NavKey } from "./components/layout/AppLayout";
import { APP_ICON_URL } from "./constants";
import { Badge } from "./components/ui/badge";
import { Button } from "./components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "./components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "./components/ui/table";

function toArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function normalizeSchemaDrift(res: SchemaDriftResponse): SchemaDriftResponse {
  const d = (res as any)?.diff ?? {};
  return {
    environment: (res as any)?.environment ?? "",
    build: ((res as any)?.build ?? {}) as Record<string, unknown>,
    diff: {
      missing_tables: toArray<string>(d.missing_tables),
      extra_tables: toArray<string>(d.extra_tables),
      tables: toArray<any>(d.tables).map((t: any) => ({
        table: t?.table ?? "",
        missing_columns: toArray<string>(t?.missing_columns),
        extra_columns: toArray<string>(t?.extra_columns),
        changed_columns: toArray<any>(t?.changed_columns).map((c: any) => ({
          column: c?.column ?? "",
          from: c?.from ?? "",
          to: c?.to ?? "",
        })),
      })),
    },
  };
}

function statusToBadgeVariant(status: string): React.ComponentProps<typeof Badge>["variant"] {
  switch (status) {
    case "success":
      return "success";
    case "failed":
      return "danger";
    case "pending":
    default:
      return "outline";
  }
}

function seedValidationVariant(issueCount: number): React.ComponentProps<typeof Badge>["variant"] {
  return issueCount === 0 ? "success" : "danger";
}

function normalizeSeedIssue(issue: any): SeedValidationIssue {
  return {
    code: issue?.code ?? "",
    path: issue?.path ?? "",
    message: issue?.message ?? "",
  };
}

function normalizeSeedEntry(seed: any): SeedInventoryEntry {
  return {
    id: seed?.id ?? "",
    kind: seed?.kind ?? "",
    table: seed?.table ?? "",
    description: seed?.description ?? "",
    filename: seed?.filename ?? "",
    row_count: seed?.row_count ?? 0,
    identity_columns: toArray<string>(seed?.identity_columns),
    delete_policy: seed?.delete_policy ?? "ignore",
    supported_environments: toArray<string>(seed?.supported_environments),
    validation_issues: toArray<any>(seed?.validation_issues).map(normalizeSeedIssue),
  };
}

function normalizeSeeds(res: SeedsResponse): SeedsResponse {
  return {
    summary: {
      total_seeds: (res as any)?.summary?.total_seeds ?? 0,
      valid_seeds: (res as any)?.summary?.valid_seeds ?? 0,
      invalid_seeds: (res as any)?.summary?.invalid_seeds ?? 0,
      required_seeds: (res as any)?.summary?.required_seeds ?? 0,
    },
    seeds: toArray<any>((res as any)?.seeds).map(normalizeSeedEntry),
  };
}

export default function App() {
  const [, setApiKeyState] = React.useState(() => getAPIKey());
  const [nav, setNav] = React.useState<NavKey>("migrations");
  const [showLogin, setShowLogin] = React.useState(() => getAPIKey().trim() === "");
  const [loginKey, setLoginKey] = React.useState(() => getAPIKey());
  const [loginError, setLoginError] = React.useState<string | null>(null);
  const [environments, setEnvironments] = React.useState<Environment[]>([]);
  const [dashboard, setDashboard] = React.useState<DashboardResponse | null>(null);
  const [runs, setRuns] = React.useState<RunRecord[]>([]);
  const [seedRuns, setSeedRuns] = React.useState<RunRecord[]>([]);
  const [seeds, setSeeds] = React.useState<SeedsResponse | null>(null);
  const [expandedSeedId, setExpandedSeedId] = React.useState<string | null>(null);
  const [seedDetailById, setSeedDetailById] = React.useState<Record<string, SeedInventoryEntry | null>>({});
  const [seedDetailLoading, setSeedDetailLoading] = React.useState<Record<string, boolean>>({});
  const [seedDetailError, setSeedDetailError] = React.useState<Record<string, string>>({});
  const [seedPlanLoading, setSeedPlanLoading] = React.useState<Record<string, boolean>>({});
  const [seedApplyLoading, setSeedApplyLoading] = React.useState<Record<string, boolean>>({});
  const [seedPlanByEnv, setSeedPlanByEnv] = React.useState<Record<string, SeedPlanResponse | null>>({});
  const [selectedSeedEnv, setSelectedSeedEnv] = React.useState<string | null>(null);
  const [seedActionError, setSeedActionError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [runLoading, setRunLoading] = React.useState<Record<string, boolean>>({});
  const [planLoading, setPlanLoading] = React.useState<Record<string, boolean>>({});
  const [planByEnv, setPlanByEnv] = React.useState<Record<string, RunPlanResponse | null>>({});
  const [planErrorByEnv, setPlanErrorByEnv] = React.useState<Record<string, string>>({});
  const [selectedPlanEnv, setSelectedPlanEnv] = React.useState<string | null>(null);
  const [driftLoading, setDriftLoading] = React.useState<Record<string, boolean>>({});
  const [driftByEnv, setDriftByEnv] = React.useState<Record<string, SchemaDriftResponse | null>>({});
  const [driftErrorByEnv, setDriftErrorByEnv] = React.useState<Record<string, string>>({});
  const [selectedDriftEnv, setSelectedDriftEnv] = React.useState<string | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const [envRes, dashRes, runsRes, seedRunsRes, seedsRes] = await Promise.all([
        getEnvironments(),
        getDashboard(),
        getRuns(20),
        getSeedRuns(20),
        getSeeds(),
      ]);
      setEnvironments(toArray(envRes?.environments));
      setDashboard({
        ...dashRes,
        rows: toArray(dashRes?.rows),
      });
      setRuns(toArray(runsRes?.runs));
      setSeedRuns(toArray(seedRunsRes?.runs));
      setSeeds(normalizeSeeds(seedsRes));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      if (e instanceof APIError && (e.status === 401 || e.status === 403)) {
        setShowLogin(true);
        setLoginError(msg);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    if (!showLogin) void refresh();
  }, [refresh]);

  async function onLoginSubmit() {
    const next = loginKey.trim();
    if (!next) {
      setLoginError("API key is required.");
      return;
    }
    setApiKeyState(next);
    setAPIKey(next);
    setLoginError(null);
    setShowLogin(false);
    await refresh();
  }

  function onLogout() {
    clearAPIKey();
    setApiKeyState("");
    setLoginKey("");
    setLoginError(null);
    setDashboard(null);
    setRuns([]);
    setSeedRuns([]);
    setSeeds(null);
    setExpandedSeedId(null);
    setSeedDetailById({});
    setSeedDetailLoading({});
    setSeedDetailError({});
    setSeedPlanLoading({});
    setSeedApplyLoading({});
    setSeedPlanByEnv({});
    setSelectedSeedEnv(null);
    setSeedActionError(null);
    setEnvironments([]);
    setNav("migrations");
    setShowLogin(true);
  }

  async function onRun(envName: string) {
    setRunLoading((m) => ({ ...m, [envName]: true }));
    try {
      const plan = await planEnvironment(envName);
      await applyEnvironment(envName, plan.plan_id);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setRunLoading((m) => ({ ...m, [envName]: false }));
      await refresh();
    }
  }

  async function onPlan(envName: string) {
    setSelectedPlanEnv(envName);
    setPlanLoading((m) => ({ ...m, [envName]: true }));
    try {
      const plan = await planEnvironment(envName);
      setPlanByEnv((m) => ({ ...m, [envName]: plan }));
      setPlanErrorByEnv((m) => ({ ...m, [envName]: "" }));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      setPlanErrorByEnv((m) => ({ ...m, [envName]: msg }));
      setPlanByEnv((m) => ({ ...m, [envName]: null }));
    } finally {
      setPlanLoading((m) => ({ ...m, [envName]: false }));
    }
  }

  function driftScore(d: SchemaDriftResponse["diff"]): number {
    const tables = toArray<any>((d as any)?.tables);
    const missingTables = toArray<string>((d as any)?.missing_tables);
    const extraTables = toArray<string>((d as any)?.extra_tables);
    const changedCols = tables.reduce((acc: number, t: any) => acc + toArray<any>(t?.changed_columns).length, 0);
    const missingCols = tables.reduce((acc: number, t: any) => acc + toArray<any>(t?.missing_columns).length, 0);
    const extraCols = tables.reduce((acc: number, t: any) => acc + toArray<any>(t?.extra_columns).length, 0);
    return missingTables.length + extraTables.length + changedCols + missingCols + extraCols;
  }

  async function onLoadDrift(envName: string) {
    setSelectedDriftEnv(envName);
    if (driftByEnv[envName]) return;
    setDriftLoading((m) => ({ ...m, [envName]: true }));
    try {
      const res = normalizeSchemaDrift(await getSchemaDrift(envName));
      setDriftByEnv((m) => ({ ...m, [envName]: res }));
      setDriftErrorByEnv((m) => ({ ...m, [envName]: "" }));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      setDriftErrorByEnv((m) => ({ ...m, [envName]: msg }));
      setDriftByEnv((m) => ({ ...m, [envName]: null }));
    } finally {
      setDriftLoading((m) => ({ ...m, [envName]: false }));
    }
  }

  async function onToggleSeed(seedId: string) {
    if (expandedSeedId === seedId) {
      setExpandedSeedId(null);
      return;
    }
    setExpandedSeedId(seedId);
    if (seedDetailById[seedId] || seedDetailLoading[seedId]) return;

    setSeedDetailLoading((m) => ({ ...m, [seedId]: true }));
    try {
      const detail = normalizeSeedEntry(await getSeedById(seedId));
      setSeedDetailById((m) => ({ ...m, [seedId]: detail }));
      setSeedDetailError((m) => ({ ...m, [seedId]: "" }));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setSeedDetailError((m) => ({ ...m, [seedId]: msg }));
      setSeedDetailById((m) => ({ ...m, [seedId]: null }));
    } finally {
      setSeedDetailLoading((m) => ({ ...m, [seedId]: false }));
    }
  }

  async function onPlanSeeds(envName: string) {
    setSelectedSeedEnv(envName);
    setSeedActionError(null);
    setSeedPlanLoading((m) => ({ ...m, [envName]: true }));
    try {
      const plan = await planSeeds(envName);
      setSeedPlanByEnv((m) => ({ ...m, [envName]: plan }));
    } catch (e) {
      setSeedActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setSeedPlanLoading((m) => ({ ...m, [envName]: false }));
    }
  }

  async function onApplySeeds(envName: string) {
    setSelectedSeedEnv(envName);
    setSeedActionError(null);
    setSeedApplyLoading((m) => ({ ...m, [envName]: true }));
    try {
      const planId = seedPlanByEnv[envName]?.plan_id;
      await applySeeds(envName, planId);
      await onPlanSeeds(envName);
      await refresh();
    } catch (e) {
      setSeedActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setSeedApplyLoading((m) => ({ ...m, [envName]: false }));
    }
  }

  const summary = dashboard?.summary;
  const seedSummary = seeds?.summary;
  const selectedDrift = selectedDriftEnv ? driftByEnv[selectedDriftEnv] : null;
  const selectedPlan = selectedPlanEnv ? planByEnv[selectedPlanEnv] : null;
  const seedRows = React.useMemo<SeedInventoryEntry[]>(() => toArray(seeds?.seeds), [seeds]);
  const selectedSeedPlan = selectedSeedEnv ? seedPlanByEnv[selectedSeedEnv] : null;

  if (showLogin) {
    return (
      <div className="min-h-screen bg-background text-foreground">
        <div className="mx-auto flex min-h-screen max-w-2xl items-center p-4">
          <Card className="w-full">
            <CardHeader>
              <CardTitle className="flex items-center gap-3 text-xl font-semibold tracking-tight">
                <img src={APP_ICON_URL} alt="PactMigrate" className="h-12 w-12 object-contain" />
                <span>PactMigrate Dashboard</span>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="text-sm text-muted-foreground">
                Enter your API key to continue. This is stored locally in your browser.
              </div>

              <div className="space-y-2">
                <label className="text-xs text-muted-foreground">API key</label>
                <input
                  value={loginKey}
                  onChange={(e) => setLoginKey(e.target.value)}
                  placeholder="pmk_..."
                  className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
                  type="password"
                  autoComplete="off"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void onLoginSubmit();
                  }}
                />
                {loginError ? <div className="text-sm whitespace-pre-wrap text-destructive">{loginError}</div> : null}
              </div>

              <div className="flex items-center justify-end gap-2">
                <Button variant="secondary" onClick={() => void onLoginSubmit()}>
                  Continue
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    );
  }

  return (
    <AppLayout
      activeNav={nav}
      onNavigate={setNav}
      onLogout={onLogout}
      onRefresh={refresh}
      loading={loading}
    >
      <div className="mx-auto max-w-7xl">
        {nav === "more" ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">More</CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              Additional tools and workflows will live here as the product expands beyond database migrations.
            </CardContent>
          </Card>
        ) : null}

        {nav === "seeds" ? (
          <>
            {error ? (
              <Card className="mt-4 border-destructive/30 bg-destructive/10">
                <CardContent className="pt-6">
                  <div className="text-sm font-semibold text-destructive">Error</div>
                  <div className="mt-1 text-sm whitespace-pre-wrap text-foreground">{error}</div>
                </CardContent>
              </Card>
            ) : null}

            <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-3">
              <Card className="lg:col-span-1">
                <CardHeader>
                  <CardTitle className="text-base">Seed Actions</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  {environments.map((env) => (
                    <div key={env.name} className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium">{env.name}</div>
                        <div className="truncate text-xs text-muted-foreground">{env.dialect}</div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => void onPlanSeeds(env.name)}
                          disabled={seedPlanLoading[env.name]}
                        >
                          {seedPlanLoading[env.name] ? "Planning..." : "Plan Seeds"}
                        </Button>
                        <Button
                          size="sm"
                          onClick={() => void onApplySeeds(env.name)}
                          disabled={seedApplyLoading[env.name]}
                        >
                          {seedApplyLoading[env.name] ? "Applying..." : "Apply Seeds"}
                        </Button>
                      </div>
                    </div>
                  ))}
                  {seedActionError ? (
                    <div className="text-sm whitespace-pre-wrap text-destructive">{seedActionError}</div>
                  ) : null}
                </CardContent>
              </Card>

              <Card className="lg:col-span-2">
                <CardHeader>
                  <CardTitle className="text-base">
                    Seed Plan Preview{selectedSeedEnv ? ` - ${selectedSeedEnv}` : ""}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {selectedSeedPlan ? (
                    <div className="space-y-3">
                      <div className="flex flex-wrap items-center gap-3 text-sm">
                        <Badge variant={selectedSeedPlan.validation_issue_count === 0 ? "success" : "danger"}>
                          {selectedSeedPlan.validation_issue_count === 0 ? "Ready to apply" : "Validation issues detected"}
                        </Badge>
                        <span className="text-muted-foreground">seed_count: {selectedSeedPlan.seed_count}</span>
                        <span className="text-muted-foreground">insert_count: {selectedSeedPlan.insert_count}</span>
                        <span className="text-muted-foreground">update_count: {selectedSeedPlan.update_count}</span>
                      </div>
                      <div className="space-y-2">
                        {selectedSeedPlan.entries.map((entry) => (
                          <div key={entry.seed_id} className="rounded-lg border border-border p-3 text-sm">
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <div className="font-medium">{entry.seed_id}</div>
                              <div className="text-xs text-muted-foreground">
                                inserts {entry.insert_count}, updates {entry.update_count}
                              </div>
                            </div>
                            <div className="mt-1 text-xs text-muted-foreground">{entry.filename}</div>
                            {entry.validation_issues.length > 0 ? (
                              <div className="mt-2 space-y-1 text-xs text-destructive">
                                {entry.validation_issues.map((issue, idx) => (
                                  <div key={`${issue.code}-${issue.path}-${idx}`}>
                                    {issue.code}: {issue.message}
                                  </div>
                                ))}
                              </div>
                            ) : (
                              <div className="mt-2 text-xs text-muted-foreground">
                                {entry.actions.length === 0 ? "No row changes detected." : `${entry.actions.length} row actions planned.`}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">Choose an environment and click `Plan Seeds`.</div>
                  )}
                </CardContent>
              </Card>
            </div>

            {seedSummary ? (
              <div className="mt-4 grid grid-cols-1 gap-4 md:grid-cols-4">
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Total Seed Files</CardTitle>
                  </CardHeader>
                  <CardContent className="text-3xl font-semibold">{seedSummary.total_seeds}</CardContent>
                </Card>
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Valid</CardTitle>
                  </CardHeader>
                  <CardContent className="text-3xl font-semibold">{seedSummary.valid_seeds}</CardContent>
                </Card>
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Needs Attention</CardTitle>
                  </CardHeader>
                  <CardContent className="text-3xl font-semibold">{seedSummary.invalid_seeds}</CardContent>
                </Card>
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Required Seeds</CardTitle>
                  </CardHeader>
                  <CardContent className="text-3xl font-semibold">{seedSummary.required_seeds}</CardContent>
                </Card>
              </div>
            ) : null}

            <div className="mt-4">
              <Card>
                <CardHeader>
                  <CardTitle className="text-base">Seed Inventory</CardTitle>
                </CardHeader>
                <CardContent>
                  {seedRows.length === 0 ? (
                    <div className="text-sm text-muted-foreground">No seed files discovered.</div>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Seed</TableHead>
                          <TableHead>Type</TableHead>
                          <TableHead>Table</TableHead>
                          <TableHead>Identity</TableHead>
                          <TableHead>Rows</TableHead>
                          <TableHead>Validation</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {seedRows.map((seed) => (
                          <React.Fragment key={seed.filename}>
                            <TableRow>
                              <TableCell>
                                <button
                                  type="button"
                                  className="min-w-0 text-left"
                                  onClick={() => void onToggleSeed(seed.id)}
                                >
                                  <div className="truncate font-medium">
                                    {expandedSeedId === seed.id ? "▼" : "▶"} {seed.id || "(missing id)"}
                                  </div>
                                  <div className="truncate text-xs text-muted-foreground">{seed.filename}</div>
                                </button>
                              </TableCell>
                              <TableCell>
                                <Badge variant="outline">{seed.kind || "unknown"}</Badge>
                              </TableCell>
                              <TableCell className="font-mono text-xs">{seed.table || "-"}</TableCell>
                              <TableCell className="font-mono text-xs">
                                {seed.identity_columns.length > 0 ? seed.identity_columns.join(", ") : "-"}
                              </TableCell>
                              <TableCell>{seed.row_count}</TableCell>
                              <TableCell>
                                <Badge variant={seedValidationVariant(seed.validation_issues.length)}>
                                  {seed.validation_issues.length === 0
                                    ? "valid"
                                    : `${seed.validation_issues.length} issue${seed.validation_issues.length === 1 ? "" : "s"}`}
                                </Badge>
                              </TableCell>
                            </TableRow>
                            {expandedSeedId === seed.id ? (
                              <TableRow>
                                <TableCell colSpan={6} className="bg-muted/30">
                                  {seedDetailLoading[seed.id] ? (
                                    <div className="text-sm text-muted-foreground">Loading seed details...</div>
                                  ) : seedDetailError[seed.id] ? (
                                    <div className="text-sm text-destructive whitespace-pre-wrap">
                                      {seedDetailError[seed.id]}
                                    </div>
                                  ) : (() => {
                                      const detail = seedDetailById[seed.id] ?? seed;
                                      return (
                                        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                                          <div className="space-y-2 text-sm">
                                            <div>
                                              <span className="font-medium">Description:</span>{" "}
                                              <span className="text-muted-foreground">{detail.description || "-"}</span>
                                            </div>
                                            <div>
                                              <span className="font-medium">File:</span>{" "}
                                              <span className="font-mono text-xs">{detail.filename}</span>
                                            </div>
                                            <div>
                                              <span className="font-medium">Delete policy:</span>{" "}
                                              <span className="font-mono text-xs">{detail.delete_policy}</span>
                                            </div>
                                            <div>
                                              <span className="font-medium">Supported environments:</span>{" "}
                                              <span className="text-muted-foreground">
                                                {detail.supported_environments.length > 0
                                                  ? detail.supported_environments.join(", ")
                                                  : "-"}
                                              </span>
                                            </div>
                                          </div>
                                          <div className="space-y-2">
                                            <div className="text-sm font-medium">Validation issues</div>
                                            {detail.validation_issues.length === 0 ? (
                                              <div className="text-sm text-muted-foreground">No validation issues.</div>
                                            ) : (
                                              <div className="space-y-2">
                                                {detail.validation_issues.map((issue, idx) => (
                                                  <div key={`${issue.code}-${issue.path}-${idx}`} className="rounded-md border border-destructive/20 bg-destructive/5 p-3 text-xs">
                                                    <div className="font-mono font-semibold">{issue.code}</div>
                                                    {issue.path ? (
                                                      <div className="mt-1 text-muted-foreground">
                                                        path: <span className="font-mono">{issue.path}</span>
                                                      </div>
                                                    ) : null}
                                                    <div className="mt-1 whitespace-pre-wrap text-foreground">{issue.message}</div>
                                                  </div>
                                                ))}
                                              </div>
                                            )}
                                          </div>
                                        </div>
                                      );
                                    })()}
                                </TableCell>
                              </TableRow>
                            ) : null}
                          </React.Fragment>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>
            </div>

            <div className="mt-4">
              <Card>
                <CardHeader>
                  <CardTitle className="text-base">Seed Run History</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead style={{ width: 160 }}>Run ID</TableHead>
                        <TableHead style={{ width: 140 }}>Environment</TableHead>
                        <TableHead style={{ width: 120 }}>Status</TableHead>
                        <TableHead style={{ width: 120 }}>Target</TableHead>
                        <TableHead style={{ width: 120 }}>Duration</TableHead>
                        <TableHead>Error</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {seedRuns.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={6}>
                            <div className="text-sm text-muted-foreground">No seed runs yet.</div>
                          </TableCell>
                        </TableRow>
                      ) : null}
                      {seedRuns.map((run) => (
                        <TableRow key={run.id}>
                          <TableCell className="font-mono text-xs">{run.id}</TableCell>
                          <TableCell>{run.environment}</TableCell>
                          <TableCell>
                            <Badge variant={run.status === "succeeded" ? "success" : "danger"}>
                              {run.status}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-sm">{run.target || "required"}</TableCell>
                          <TableCell className="text-sm">{run.duration_ms}ms</TableCell>
                          <TableCell className="max-w-xl">
                            {run.error ? (
                              <span className="block max-h-16 overflow-hidden text-ellipsis whitespace-pre-wrap text-xs">
                                {run.error}
                              </span>
                            ) : (
                              <span className="text-sm text-muted-foreground">-</span>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </div>
          </>
        ) : null}

        {nav === "migrations" ? (
          <>
        {error ? (
          <Card className="mt-4 border-destructive/30 bg-destructive/10">
            <CardContent className="pt-6">
              <div className="text-sm font-semibold text-destructive">Error</div>
              <div className="mt-1 text-sm whitespace-pre-wrap text-foreground">{error}</div>
            </CardContent>
          </Card>
        ) : null}

        {summary ? (
          <div className="mt-4 grid grid-cols-1 gap-4 md:grid-cols-3">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Total Migrations</CardTitle>
              </CardHeader>
              <CardContent className="text-3xl font-semibold">{summary.total_migrations}</CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Migrations Failed</CardTitle>
              </CardHeader>
              <CardContent className="text-3xl font-semibold">{summary.migrations_failed}</CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Last Event</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-3xl font-semibold">{summary.last_event_human_hint || "-"}</div>
                <div className="mt-1 text-sm text-muted-foreground">{summary.last_event_at || "-"}</div>
              </CardContent>
            </Card>
          </div>
        ) : null}

        <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-3">
          <Card className="lg:col-span-1">
            <CardHeader>
              <CardTitle className="text-base">Environments</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {environments.length === 0 ? (
                <div className="text-sm text-muted-foreground">No environments configured.</div>
              ) : null}
              {environments.map((env) => (
                <div key={env.name} className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium">{env.name}</div>
                    <div className="truncate text-xs text-muted-foreground">
                      {env.dialect} / {env.tableName}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => void onPlan(env.name)}
                      disabled={planLoading[env.name]}
                    >
                      {planLoading[env.name] ? "Planning..." : "Plan"}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => void onLoadDrift(env.name)}
                      disabled={driftLoading[env.name]}
                    >
                      {driftLoading[env.name] ? "Checking..." : "Drift"}
                    </Button>
                    <Button
                      size="sm"
                      onClick={() => void onRun(env.name)}
                      disabled={runLoading[env.name]}
                    >
                      {runLoading[env.name] ? "Running..." : "Run"}
                    </Button>
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle className="text-base">Migration Status</CardTitle>
            </CardHeader>
            <CardContent>
              {dashboard ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead style={{ width: "35%" }}>Migration</TableHead>
                      {environments.map((env) => (
                        <TableHead key={env.name}>{env.name}</TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {dashboard.rows.map((row) => (
                      <TableRow key={row.id}>
                        <TableCell>
                          <div className="min-w-0">
                            <div className="truncate font-medium">{row.name}</div>
                            <div className="truncate text-xs text-muted-foreground">
                              {row.filename}
                            </div>
                          </div>
                        </TableCell>
                        {environments.map((env) => {
                          const status = row.statuses[env.name] ?? "pending";
                          return (
                            <TableCell key={env.name}>
                              <Badge variant={statusToBadgeVariant(status)}>{status}</Badge>
                            </TableCell>
                          );
                        })}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : (
                <div className="text-sm text-muted-foreground">Loading dashboard...</div>
              )}
            </CardContent>
          </Card>
        </div>

        {selectedPlanEnv ? (
          <div className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Plan Preview — {selectedPlanEnv}</CardTitle>
              </CardHeader>
              <CardContent>
                {planLoading[selectedPlanEnv] ? (
                  <div className="text-sm text-muted-foreground">Loading plan…</div>
                ) : planErrorByEnv[selectedPlanEnv] ? (
                  <div className="space-y-2">
                    <div className="text-sm font-semibold text-destructive">Failed to load plan</div>
                    <div className="text-sm whitespace-pre-wrap">{planErrorByEnv[selectedPlanEnv]}</div>
                  </div>
                ) : selectedPlan ? (
                  <div className="space-y-3">
                    <div className="flex flex-wrap items-center gap-2 text-sm">
                      <Badge variant={selectedPlan.planned_count === 0 ? "success" : "outline"}>
                        {selectedPlan.planned_count === 0 ? "Up to date" : "Pending migrations"}
                      </Badge>
                      <span className="text-muted-foreground">
                        plan_id: <span className="font-mono text-xs">{selectedPlan.plan_id}</span>
                      </span>
                      <span className="text-muted-foreground">planned_count: {selectedPlan.planned_count}</span>
                    </div>

                    {selectedPlan.remaining_pending.length > 0 ? (
                      <div className="rounded-lg border border-border">
                        <div className="border-b border-border px-3 py-2 text-sm font-medium">Remaining pending</div>
                        <div className="max-h-64 overflow-auto p-3">
                          <ul className="space-y-1 font-mono text-xs">
                            {selectedPlan.remaining_pending.map((k) => (
                              <li key={k} className="rounded bg-muted px-2 py-1">
                                {k}
                              </li>
                            ))}
                          </ul>
                        </div>
                      </div>
                    ) : (
                      <div className="text-sm text-muted-foreground">No pending migrations.</div>
                    )}
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">No plan data available.</div>
                )}
              </CardContent>
            </Card>
          </div>
        ) : null}

        {selectedDriftEnv ? (
          <div className="mt-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Schema Drift — {selectedDriftEnv}</CardTitle>
              </CardHeader>
              <CardContent>
                {driftLoading[selectedDriftEnv] ? (
                  <div className="text-sm text-muted-foreground">Loading drift…</div>
                ) : driftErrorByEnv[selectedDriftEnv] ? (
                  <div className="space-y-2">
                    <div className="text-sm font-semibold text-destructive">Failed to load drift</div>
                    <div className="text-sm whitespace-pre-wrap">{driftErrorByEnv[selectedDriftEnv]}</div>
                    <div className="text-xs text-muted-foreground">
                      Hint: set <span className="font-mono">scratch_dsn</span> for this environment in
                      <span className="font-mono"> apps/server/config.json</span> and restart the server.
                    </div>
                  </div>
                ) : selectedDrift ? (
                  <div className="space-y-3">
                  <div className="flex flex-wrap items-center gap-2 text-sm">
                    <Badge variant={driftScore(selectedDrift.diff) === 0 ? "success" : "danger"}>
                      {driftScore(selectedDrift.diff) === 0 ? "No drift" : "Drift detected"}
                    </Badge>
                    <span className="text-muted-foreground">
                      Missing tables: {selectedDrift.diff.missing_tables.length}, extra tables:{" "}
                      {selectedDrift.diff.extra_tables.length}, changed tables:{" "}
                      {selectedDrift.diff.tables.length}
                    </span>
                  </div>

                  {selectedDrift.diff.missing_tables.length > 0 ? (
                    <div>
                      <div className="text-sm font-medium">Missing tables (expected, not found)</div>
                      <div className="mt-1 flex flex-wrap gap-2">
                        {selectedDrift.diff.missing_tables.map((t) => (
                          <Badge key={t} variant="outline">
                            {t}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  ) : null}

                  {selectedDrift.diff.extra_tables.length > 0 ? (
                    <div>
                      <div className="text-sm font-medium">Extra tables (found, not expected)</div>
                      <div className="mt-1 flex flex-wrap gap-2">
                        {selectedDrift.diff.extra_tables.map((t) => (
                          <Badge key={t} variant="outline">
                            {t}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  ) : null}

                  {selectedDrift.diff.tables.length > 0 ? (
                    <div>
                      <div className="text-sm font-medium">Table diffs</div>
                      <div className="mt-2 space-y-3">
                        {selectedDrift.diff.tables.map((t) => (
                          <div key={t.table} className="rounded-lg border border-border p-3">
                            <div className="flex items-center justify-between">
                              <div className="text-sm font-semibold">{t.table}</div>
                              <div className="text-xs text-muted-foreground">
                                {t.missing_columns.length} missing, {t.extra_columns.length} extra,{" "}
                                {t.changed_columns.length} changed
                              </div>
                            </div>

                            {t.missing_columns.length > 0 ? (
                              <div className="mt-2">
                                <div className="text-xs font-medium text-muted-foreground">Missing columns</div>
                                <div className="mt-1 flex flex-wrap gap-2">
                                  {t.missing_columns.map((c) => (
                                    <Badge key={c} variant="outline">
                                      {c}
                                    </Badge>
                                  ))}
                                </div>
                              </div>
                            ) : null}

                            {t.extra_columns.length > 0 ? (
                              <div className="mt-2">
                                <div className="text-xs font-medium text-muted-foreground">Extra columns</div>
                                <div className="mt-1 flex flex-wrap gap-2">
                                  {t.extra_columns.map((c) => (
                                    <Badge key={c} variant="outline">
                                      {c}
                                    </Badge>
                                  ))}
                                </div>
                              </div>
                            ) : null}

                            {t.changed_columns.length > 0 ? (
                              <div className="mt-2">
                                <div className="text-xs font-medium text-muted-foreground">Changed columns</div>
                                <div className="mt-2 space-y-2">
                                  {t.changed_columns.map((c) => (
                                    <div key={c.column} className="rounded-md bg-muted p-2 text-xs">
                                      <div className="font-mono font-semibold">{c.column}</div>
                                      <div className="mt-1 grid grid-cols-1 gap-1 md:grid-cols-2">
                                        <div>
                                          <div className="text-muted-foreground">expected</div>
                                          <div className="font-mono">{c.from}</div>
                                        </div>
                                        <div>
                                          <div className="text-muted-foreground">actual</div>
                                          <div className="font-mono">{c.to}</div>
                                        </div>
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            ) : null}
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : null}
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">No drift data available.</div>
                )}
              </CardContent>
            </Card>
          </div>
        ) : null}

        <div className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Run History</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead style={{ width: 160 }}>Run ID</TableHead>
                    <TableHead style={{ width: 160 }}>Environment</TableHead>
                    <TableHead style={{ width: 120 }}>Status</TableHead>
                    <TableHead style={{ width: 140 }}>Duration</TableHead>
                    <TableHead>Error</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {runs.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5}>
                        <div className="text-sm text-muted-foreground">No runs yet.</div>
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {runs.map((r) => (
                    <TableRow key={r.id}>
                      <TableCell className="font-mono text-xs">{r.id}</TableCell>
                      <TableCell>{r.environment}</TableCell>
                      <TableCell>
                        <Badge
                          variant={r.status === "succeeded" ? "success" : "danger"}
                        >
                          {r.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm">{r.duration_ms}ms</TableCell>
                      <TableCell className="max-w-xl">
                        {r.error ? (
                          <span className="block max-h-16 overflow-hidden text-ellipsis whitespace-pre-wrap text-xs">
                            {r.error}
                          </span>
                        ) : (
                          <span className="text-sm text-muted-foreground">-</span>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
          </>
        ) : null}
      </div>
    </AppLayout>
  );
}

