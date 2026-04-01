# PactMigrate Seeder Spec

This document defines the Phase 0 seeder contract for PactMigrate.

Phase 0 does not implement seed execution yet. It locks the model, file format, config shape, and package boundaries so later phases can add inventory, plan/apply, drift detection, and dashboard editing without reworking the basics.

## Goals

- Treat seed data as versioned repo state instead of ad hoc SQL scripts.
- Keep static seeds idempotent by default.
- Make the static seed format easy to validate, diff, and eventually edit in the dashboard.
- Reuse the existing PactMigrate architecture:
  - `packages/core` for shared seeder logic
  - `apps/pactmigrate-cli` for file layout and operator workflows
  - `apps/server` for future API exposure
  - `apps/dashboard` for future visibility and editing

## Phase 0 Decisions

- Static file-backed seeds use `JSON` only.
- Seed assets live under `apps/pactmigrate-cli/seeds/`.
- Shared seeder logic belongs in `packages/core`.
- `required` seeds use one file per table.
- Static seed files must declare explicit `identity.columns`.
- Initial delete policy is `ignore` only.
- `required` seeds are manual only in the first executable release.
- `persona` and `capture` are modeled now but not executable in early phases.

## Seeder Taxonomy

### `required`

Static file-backed data the application depends on to function correctly.

Examples:

- roles
- currencies
- countries
- status catalogs

Characteristics:

- file-backed
- versioned in git
- safe to apply repeatedly
- included in drift comparisons later

### `persona`

Generated test/demo datasets driven by code.

Examples:

- 500 users with orders
- demo storefront data

Characteristics:

- code-backed
- parameterized
- manually triggered
- not part of the initial Phase 1 and Phase 2 execution scope

### `capture`

Seed artifacts exported from a curated live/local database state.

Characteristics:

- file-backed
- manually created
- reviewable before reuse
- not part of the initial execution scope

## Disk Layout

Canonical root:

```text
apps/pactmigrate-cli/seeds/
```

Planned layout:

```text
apps/pactmigrate-cli/seeds/
  required/
    <domain>/
      <name>.seed.json
  captures/
    <domain>/
      <timestamp>-<name>.seed.json
  personas/
    <persona-id>/
      definition.json
```

Phase 0 and early execution phases only require the `required/` layout to be populated.

## Static Seed File Contract

Static seeds are JSON files with one file per target table.

Example:

```json
{
  "version": 1,
  "kind": "required",
  "id": "core.roles",
  "table": "roles",
  "description": "System roles required by the app",
  "identity": {
    "columns": ["code"]
  },
  "delete_policy": "ignore",
  "rows": [
    {
      "code": "admin",
      "name": "Administrator"
    },
    {
      "code": "viewer",
      "name": "Viewer"
    }
  ]
}
```

### Required fields

- `version`
- `kind`
- `id`
- `table`
- `identity.columns`
- `rows`

### Optional fields

- `description`
- `depends_on`
- `tags`
- `delete_policy`

### Field rules

#### `version`

- integer
- current value must be `1`

#### `kind`

- must be one of:
  - `required`
  - `capture`
- static execution in early phases only supports `required`

Note:
- `persona` is part of the product taxonomy, but it is code-backed rather than static file-backed in the initial design

#### `id`

- stable logical identifier
- recommended format: `<domain>.<name>`
- examples:
  - `core.roles`
  - `core.currencies`

#### `table`

- exact target table name in the database
- one static seed file targets one table only

#### `identity.columns`

- non-empty array of column names
- identifies a row for upsert and drift comparison
- must be explicitly declared in every static seed file
- every row must include all identity columns

#### `delete_policy`

Allowed values:

- `ignore`

Future policies may be added later, but Phase 0 locks execution behavior to `ignore` only.

#### `rows`

- array of objects
- each object represents one logical row
- row keys should match target column names

## Validation Rules

Phase 0 validation contract:

- top-level JSON must be an object
- unknown top-level fields should fail validation
- `version` must be `1`
- `kind` must be valid for a static file-backed seed
- `id` must be non-empty
- `table` must be non-empty
- `identity.columns` must be present and non-empty
- identity column names must be unique within the array
- `rows` must be an array
- every row must be an object
- every row must include all declared identity columns
- duplicate identity tuples inside one file are invalid
- `delete_policy` defaults to `ignore` if omitted

Additional runtime validation in later phases:

- `identity.columns` must match a usable primary key or unique constraint in the target DB
- row fields should map to real table columns

## Identity Model

PactMigrate does not infer row identity solely from the database.

Instead:

- each static seed file declares `identity.columns`
- execution logic later validates that those columns correspond to a real PK or unique constraint

Why:

- better drift detection
- safer upsert targeting
- less ambiguity when tables have multiple unique constraints

## Delete Semantics

Initial behavior is intentionally conservative.

- `delete_policy: "ignore"` means extra rows found in the database are not deleted
- initial plan/apply phases support insert/update only
- destructive reconciliation belongs to a later phase

## Execution Model

For the initial executable release:

- `required` seeds are manual only
- they do not automatically run after migrations
- the operator must explicitly plan/apply them through CLI or dashboard in later phases

This keeps first-release seeding predictable and low-risk.

## Audit Model

Seed runs should align with the existing migration run concept instead of inventing a separate audit system.

Recommended future run shape:

- `kind`: `migration` or `seed`
- `seed_kind`: `required`, `persona`, or `capture`
- `seed_id`
- `target_table`
- `mode`: `plan` or `apply`
- `inserted_count`
- `updated_count`
- later `deleted_count`
- `status`
- `actor_id`
- `started_at`
- `finished_at`
- `duration_ms`
- `error`

## Package Ownership

### `packages/core`

Owns:

- seed types
- seed loading
- seed validation
- future seed planning and apply logic
- future SQL dialect upsert generation

### `apps/pactmigrate-cli`

Owns:

- seed asset layout under `seeds/`
- future CLI commands for listing/planning/applying seeds

### `apps/server`

Owns:

- future REST handlers for seed inventory, plans, applies, and drift

### `apps/dashboard`

Owns:

- future seed inventory UI
- future plan/apply UI
- future editing and sync views

## Configuration Shape

The initial config contract should reserve a top-level `seeds` section.

Recommended shape:

```json
{
  "seeds": {
    "enabled": true,
    "dir": "apps/pactmigrate-cli/seeds"
  }
}
```

Future fields may include:

- `auto_apply_required_after_migration`
- `allow_personas`
- `allow_capture`

For early phases:

- `enabled` defaults to `true`
- `dir` defaults to `apps/pactmigrate-cli/seeds`
- execution still remains manual only

## Initial Examples

Phase 0 should include at least:

- `apps/pactmigrate-cli/seeds/required/core/roles.seed.json`
- `apps/pactmigrate-cli/seeds/required/core/currencies.seed.json`

These are examples of the canonical static seed shape and provide fixtures for future loader/validator work.
