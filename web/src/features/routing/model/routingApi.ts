import { z } from "zod";

import { request, requestNoContent } from "platform/api/http";

export type ModelMapping = {
  id: string;
  route_key: string;
  provider: string;
  model: string;
  enabled: boolean;
};

export type VerdictProvider = {
  id: string;
  name: string;
  endpoint: string;
  credential_ref?: string;
  mode: "inline" | "async_shadow";
  enabled: boolean;
};

export type GatewayClient = {
  id: string;
  name: string;
  status: "enabled" | "disabled" | "revoked";
  key_prefix?: string;
  notes?: string;
  created_at?: string;
  updated_at?: string;
  revoked_at?: string;
};

export type GatewayClientCreateResponse = {
  client: GatewayClient;
  key: string;
};

export type GatewayRouteLimit = {
  route_key: string;
  requests_per_window: number;
  window_seconds: number;
  max_concurrent_requests: number;
  max_body_bytes: number;
  enabled: boolean;
};

export type GatewayClientRouteLimit = GatewayRouteLimit & {
  client_id: string;
};

const modelMappingSchema = z.object({
  id: z.string(),
  route_key: z.string(),
  provider: z.string(),
  model: z.string(),
  enabled: z.boolean()
});

const verdictProviderSchema = z.object({
  id: z.string(),
  name: z.string(),
  endpoint: z.string(),
  credential_ref: z.string().optional(),
  mode: z.enum(["inline", "async_shadow"]),
  enabled: z.boolean()
});

const gatewayClientSchema = z.object({
  id: z.string(),
  name: z.string(),
  status: z.enum(["enabled", "disabled", "revoked"]),
  key_prefix: z.string().optional(),
  notes: z.string().optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
  revoked_at: z.string().optional()
});

const gatewayClientCreateResponseSchema = z.object({
  client: gatewayClientSchema,
  key: z.string()
});

const gatewayRouteLimitSchema = z.object({
  route_key: z.string(),
  requests_per_window: z.number(),
  window_seconds: z.number(),
  max_concurrent_requests: z.number(),
  max_body_bytes: z.number(),
  enabled: z.boolean()
});

const gatewayClientRouteLimitSchema = gatewayRouteLimitSchema.extend({
  client_id: z.string()
});

export async function listModelMappings(): Promise<{ items: ModelMapping[] }> {
  return request("/api/routing/model-mappings", z.object({ items: z.array(modelMappingSchema) }));
}

export async function putModelMapping(id: string, mapping: ModelMapping): Promise<ModelMapping> {
  return request(`/api/routing/model-mappings/${encodeURIComponent(id)}`, modelMappingSchema, {
    method: "PUT",
    body: JSON.stringify(mapping)
  });
}

export async function listVerdictProviders(): Promise<{ items: VerdictProvider[] }> {
  return request("/api/providers/verdict", z.object({ items: z.array(verdictProviderSchema) }));
}

export async function putVerdictProvider(id: string, provider: VerdictProvider): Promise<VerdictProvider> {
  return request(`/api/providers/verdict/${encodeURIComponent(id)}`, verdictProviderSchema, {
    method: "PUT",
    body: JSON.stringify(provider)
  });
}

export async function listGatewayClients(): Promise<{ items: GatewayClient[] }> {
  return request("/api/gateway-clients", z.object({ items: z.array(gatewayClientSchema) }));
}

export async function createGatewayClient(input: { name: string; notes?: string }): Promise<GatewayClientCreateResponse> {
  return request("/api/gateway-clients", gatewayClientCreateResponseSchema, {
    method: "POST",
    body: JSON.stringify(input)
  });
}

export async function patchGatewayClient(id: string, client: GatewayClient): Promise<GatewayClient> {
  return request(`/api/gateway-clients/${encodeURIComponent(id)}`, gatewayClientSchema, {
    method: "PATCH",
    body: JSON.stringify(client)
  });
}

export async function revokeGatewayClient(id: string): Promise<GatewayClient> {
  return request(`/api/gateway-clients/${encodeURIComponent(id)}/revoke`, gatewayClientSchema, {
    method: "POST",
    body: JSON.stringify({})
  });
}

export async function listGatewayRouteLimits(): Promise<{ items: GatewayRouteLimit[] }> {
  return request("/api/gateway-limits/routes", z.object({ items: z.array(gatewayRouteLimitSchema) }));
}

export async function putGatewayRouteLimit(routeKey: string, limit: GatewayRouteLimit): Promise<GatewayRouteLimit> {
  return request(`/api/gateway-limits/routes/${encodeURIComponent(routeKey)}`, gatewayRouteLimitSchema, {
    method: "PUT",
    body: JSON.stringify(limit)
  });
}

export async function listGatewayClientRouteLimits(clientID: string): Promise<{ items: GatewayClientRouteLimit[] }> {
  return request(`/api/gateway-limits/clients/${encodeURIComponent(clientID)}/routes`, z.object({ items: z.array(gatewayClientRouteLimitSchema) }));
}

export async function putGatewayClientRouteLimit(clientID: string, routeKey: string, limit: GatewayClientRouteLimit): Promise<GatewayClientRouteLimit> {
  return request(`/api/gateway-limits/clients/${encodeURIComponent(clientID)}/routes/${encodeURIComponent(routeKey)}`, gatewayClientRouteLimitSchema, {
    method: "PUT",
    body: JSON.stringify(limit)
  });
}

export async function deleteGatewayClientRouteLimit(clientID: string, routeKey: string): Promise<void> {
  return requestNoContent(`/api/gateway-limits/clients/${encodeURIComponent(clientID)}/routes/${encodeURIComponent(routeKey)}`, {
    method: "DELETE"
  });
}
