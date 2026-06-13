import type { GatewayClientRouteLimit, GatewayRouteLimit } from "./routingApi";

export type EffectiveLimit = {
  route_key: string;
  source: "default" | "override";
  limit: GatewayRouteLimit;
};

export const starterRouteLimit = {
  route_key: "chat",
  requests_per_window: 60,
  window_seconds: 60,
  max_concurrent_requests: 10,
  max_body_bytes: 1048576,
  enabled: true
} satisfies GatewayRouteLimit;

export function buildEffectiveLimits(defaults: GatewayRouteLimit[], overrides: GatewayClientRouteLimit[]): EffectiveLimit[] {
  const defaultByRoute = new Map(defaults.map((limit) => [limit.route_key, limit]));
  const overrideByRoute = new Map(overrides.map((limit) => [limit.route_key, limit]));
  const routeKeys = [...defaultByRoute.keys()].sort();

  return routeKeys.map((routeKey) => {
    const override = overrideByRoute.get(routeKey);
    if (override?.enabled) return { route_key: routeKey, source: "override", limit: override };
    return { route_key: routeKey, source: "default", limit: defaultByRoute.get(routeKey)! };
  });
}

export function limitSummary(limit: GatewayRouteLimit, t: (key: string, values?: Record<string, string | number>) => string) {
  return t("access.limitSummary", {
    requests: limit.requests_per_window,
    window: limit.window_seconds,
    concurrent: limit.max_concurrent_requests,
    bytes: limit.max_body_bytes
  });
}

export function numberValue(value: string) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.max(0, Math.floor(parsed)) : 0;
}
