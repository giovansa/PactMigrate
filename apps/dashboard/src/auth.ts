const STORAGE_KEY = "pactmigrate.api_key";

export function getAPIKey(): string {
  const fromEnv = (import.meta as any)?.env?.VITE_API_KEY as string | undefined;
  if (fromEnv && fromEnv.trim() !== "") return fromEnv.trim();
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    return (v ?? "").trim();
  } catch {
    return "";
  }
}

export function setAPIKey(key: string) {
  try {
    localStorage.setItem(STORAGE_KEY, key.trim());
  } catch {
    // ignore
  }
}

export function clearAPIKey() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // ignore
  }
}

