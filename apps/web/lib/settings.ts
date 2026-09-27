import "server-only";

import type { components } from "@/lib/api/generated";
import { apiRequest, authenticatedRequest } from "@/lib/api/server";

async function json<T>(response: Promise<Response | null>) {
  const result = await response.catch(() => null);
  return result?.ok ? ((await result.json()) as T) : null;
}

export async function getSettingsData() {
  const [
    preferences,
    positions,
    sourceCatalog,
    sourcePreferences,
    telegram,
    notifications,
  ] = await Promise.all([
    json<components["schemas"]["Preferences"]>(
      authenticatedRequest("/api/v1/profile/preferences"),
    ),
    json<components["schemas"]["PositionsOutputBody"]>(
      authenticatedRequest("/api/v1/profile/positions"),
    ),
    json<components["schemas"]["SourceCatalogOutputBody"]>(
      apiRequest("/api/v1/sources"),
    ),
    json<components["schemas"]["SourcePreferencesOutputBody"]>(
      authenticatedRequest("/api/v1/profile/sources"),
    ),
    json<components["schemas"]["TelegramOutputBody"]>(
      authenticatedRequest("/api/v1/telegram"),
    ),
    json<components["schemas"]["NotificationPreferences"]>(
      authenticatedRequest("/api/v1/telegram/preferences"),
    ),
  ]);
  return {
    preferences,
    positions: positions?.positions ?? [],
    sources: sourceCatalog?.sources ?? [],
    sourcePreferences: sourcePreferences?.sources ?? [],
    telegram,
    notifications,
  };
}
