# The PactMigrate Seeder: State over Scripts

## 1. The Core Philosophy: "State over Scripts"

Traditional seeders are often "fire and forget" SQL files that break if run twice. The PactMigrate Seeder treats data as code that can be consistently adjusted.

- **Idempotency by Default:** Every seed operation uses Upsert logic (`INSERT ... ON CONFLICT`). If the record exists, update it; if not, create it.
- **Versioned Seeds:** Just like migrations, seeds can be versioned and tracked.
- **Source of Truth:** The JSON/YAML files in your repo are the "truth." The Dashboard is the "editor."

## 2. Key Seeding Strategies

### A. Static Lookups (The "Required" Pact)

- **Purpose:** Essential data that the app needs to function (e.g., roles, currencies, countries).
- **Format:** Versioned JSON files.
- **Behavior:** Automatically applied after successful migrations.

### B. Development Personas (The "Populator")

- **Purpose:** Large batches of mock data for testing (e.g., "500 Users with Orders").
- **Format:** Go-based logic using a Faker Engine (like `gofakeit`).
- **Behavior:** Triggered manually via CLI or Dashboard "Populate" button.

### C. The "Capture" Flow (The "Time Machine")

- **Purpose:** The ability to take a hand‑curated set of data from a developer's local DB and "export" it into a shareable seed file.
- **Workflow:** Developer sets up complex data in DB → Clicks "Capture State" in Dashboard → Backend generates a `seed_capture_[timestamp].json` file.

## 3. Dashboard Integration (UI Features)

| Feature               | Description                                                              | UI Component                |
|-----------------------|--------------------------------------------------------------------------|-----------------------------|
| Data Grid Editor      | View and edit JSON seed files directly in the browser.                   | Airtable‑style Editable Grid |
| Seed Drift Check      | Flags if a "Seeded" row has been manually changed in the DB.             | "Out of Sync" Warning Badge  |
| Persona Toggle        | Quickly switch between "Load Test Data" and "Clean Demo Data."           | Dropdown / Toggle Switch     |
| Faker Control         | Generate N rows of random data for a specific table.                     | "Generate" Modal with sliders |

## 4. Technical Implementation Logic

### The Upsert Driver

To allow "consistent adjustment," the backend must intelligently handle conflicts.

- **PostgreSQL:** `INSERT ... ON CONFLICT (pk) DO UPDATE SET ...`
- **MySQL:** `INSERT ... ON DUPLICATE KEY UPDATE ...`
- **Logic:** The library inspects the table's Primary Key/Unique constraints automatically to build the Upsert query.

### The SeedSync Engine

A background worker that:

1. Reads the current seed file.
2. Compares it to the database table.
3. Calculates the Delta (what needs to be Added, Updated, or Deleted).
4. Reports the "Sync Status" to the Dashboard.
