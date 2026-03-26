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

