import type { TrafficEventSummary } from "./traffic";

export type ConsolePostureSummary = {
  activeKeysCount: number;
  healthIsError: boolean;
  healthIsLoading: boolean;
  healthState: string;
  mappingCount: number;
  policyBundleCount: number;
  providerCount: number;
  spoolDepth: number | null;
  trafficSummary: TrafficEventSummary;
};
