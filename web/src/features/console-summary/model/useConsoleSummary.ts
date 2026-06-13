import { useQuery } from "@tanstack/react-query";

import { getConsoleSummary, type ConsoleSummaryResponse } from "platform/api/consoleSummary";
import { emptyTrafficEventSummary } from "platform/api/trafficSummary";
import { queryKeys } from "platform/query/keys";
import type { ConsolePostureSummary } from "shared/types/console";

export function useConsoleSummary() {
  return useQuery({
    queryKey: queryKeys.consoleSummary,
    queryFn: ({ signal }) => getConsoleSummary(signal),
    retry: 1,
    refetchInterval: 15_000
  });
}

export function useConsolePostureSummary() {
  const query = useConsoleSummary();

  return {
    query,
    summary: consolePostureFromSummary(query.data, query)
  };
}

function consolePostureFromSummary(summary: ConsoleSummaryResponse | undefined, state: { isError: boolean; isLoading: boolean }): ConsolePostureSummary {
  return {
    activeKeysCount: summary?.policy.active_bundle_key_count ?? 0,
    healthIsError: state.isError,
    healthIsLoading: state.isLoading,
    healthState: state.isError ? "unhealthy" : summary ? "ok" : "loading",
    mappingCount: summary?.routing.model_mapping_count ?? 0,
    policyBundleCount: summary?.policy.bundle_count ?? 0,
    providerCount: summary?.routing.verdict_provider_count ?? 0,
    spoolDepth: summary?.storage.available === false ? null : summary?.storage.depth ?? 0,
    trafficSummary: summary?.traffic ?? emptyTrafficEventSummary
  };
}
