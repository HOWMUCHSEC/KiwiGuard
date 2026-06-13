export const queryKeys = {
  activeConfig: ["active-config"] as const,
  consoleSummary: ["console-summary"] as const,
  gatewayClientRouteLimits: (clientID: string) => ["gateway-client-route-limits", clientID] as const,
  gatewayClients: ["gateway-clients"] as const,
  gatewayRouteLimits: ["gateway-route-limits"] as const,
  health: ["health"] as const,
  modelMappings: ["model-mappings"] as const,
  policyBundles: ["policy-bundles"] as const,
  trafficEvents: (filters: { direction?: string; limit?: number; provider_id?: string; route_id?: string; status?: string }) =>
    ["traffic-events", filters.route_id ?? "", filters.provider_id ?? "", filters.direction ?? "", filters.status ?? "", filters.limit ?? 25] as const,
  trafficSpool: ["traffic-spool"] as const,
  verdictProviders: ["verdict-providers"] as const
};
