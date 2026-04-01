export type MigrationStatus = "pending" | "success" | "failed";

export type Environment = {
  name: string;
  dialect: "postgres" | "mysql" | string;
  tableName: string;
};

export type DashboardRow = {
  id: string;
  name: string;
  version: string;
  statuses: Record<string, MigrationStatus>;
  filename: string;
  timestamp: string;
};

export type DashboardResponse = {
  summary: {
    total_migrations: number;
    active_environments: number;
    environment_names: string[];
    migrations_failed: number;
    last_event_at: string;
    last_event_human_hint: string;
  };
  rows: DashboardRow[];
};

export type RunRecord = {
  id: string;
  environment: string;
  kind?: string;
  target?: string;
  status: "succeeded" | "failed";
  started_at: string;
  finished_at: string;
  duration_ms: number;
  planned_count: number;
  applied_count: number;
  remaining_count: number;
  error?: string;
};

export type RunsResponse = {
  runs: RunRecord[];
};

export type RunPlanResponse = {
  mode: "plan";
  plan_id: string;
  environment: string;
  planned_count: number;
  remaining_pending: string[];
};

export type SchemaDriftResponse = {
  environment: string;
  build: Record<string, unknown>;
  diff: {
    missing_tables: string[];
    extra_tables: string[];
    tables: Array<{
      table: string;
      missing_columns: string[];
      extra_columns: string[];
      changed_columns: Array<{
        column: string;
        from: string;
        to: string;
      }>;
    }>;
  };
};

export type SeedSummary = {
  total_seeds: number;
  valid_seeds: number;
  invalid_seeds: number;
  required_seeds: number;
};

export type SeedInventoryEntry = {
  id: string;
  kind: string;
  table: string;
  description?: string;
  filename: string;
  row_count: number;
  identity_columns: string[];
  delete_policy: string;
  supported_environments: string[];
  validation_issues: SeedValidationIssue[];
};

export type SeedsResponse = {
  summary: SeedSummary;
  seeds: SeedInventoryEntry[];
};

export type SeedValidationIssue = {
  code: string;
  path: string;
  message: string;
};

export type SeedPlanEntry = {
  seed_id: string;
  filename: string;
  table: string;
  row_count: number;
  insert_count: number;
  update_count: number;
  validation_issues: SeedValidationIssue[];
  actions: Array<{
    action: string;
    identity: Record<string, unknown>;
  }>;
};

export type SeedPlanResponse = {
  kind: "seed";
  mode: "plan";
  plan_id: string;
  request_id: string;
  environment: string;
  seed_count: number;
  insert_count: number;
  update_count: number;
  validation_issue_count: number;
  entries: SeedPlanEntry[];
};

export type SeedApplyResponse = {
  kind: "seed";
  status: "succeeded" | "failed";
  mode: "apply";
  environment?: string;
  seed_count?: number;
  insert_count?: number;
  update_count?: number;
  applied_count?: number;
  entries?: SeedPlanEntry[];
  error?: string;
};

