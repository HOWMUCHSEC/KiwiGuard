export type TrafficSummarySnapshot = {
  total: number;
  blocked: number;
  upstream_errors: number;
  fallbacks: number;
};

export type TrafficEventSummary = TrafficSummarySnapshot;
