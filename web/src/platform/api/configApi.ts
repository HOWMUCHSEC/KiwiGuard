import { z } from "zod";

import { request } from "./http";

export const healthResponseSchema = z.object({
  status: z.enum(["ok", "degraded", "unhealthy"]),
  version: z.string(),
  timestamp: z.string(),
  checks: z
    .record(
      z.string(),
      z.object({
        status: z.string(),
        reason: z.string().optional(),
        depth: z.number().optional(),
        bytes: z.number().optional(),
        max_bytes: z.number().optional(),
        oldest_age_seconds: z.number().optional(),
        overflow_count: z.number().optional()
      })
    )
    .optional()
});

const configStatusSchema = z.object({
  active_policy_bundle_keys: z.array(z.string()),
  policy_snapshot_hash: z.string()
});

export type HealthResponse = z.infer<typeof healthResponseSchema>;
export type ConfigStatusResponse = z.infer<typeof configStatusSchema>;

export async function getHealth(): Promise<HealthResponse> {
  return request("/api/healthz", healthResponseSchema, undefined, { acceptErrorBody: true });
}

export async function getActiveConfig(): Promise<ConfigStatusResponse> {
  return request("/api/config/active", configStatusSchema);
}
