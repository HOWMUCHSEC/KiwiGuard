import type { TrafficEventSummary } from "shared/types/traffic";

export type { TrafficEventSummary } from "shared/types/traffic";

export const emptyTrafficEventSummary: TrafficEventSummary = {
  total: 0,
  blocked: 0,
  upstream_errors: 0,
  fallbacks: 0
};
