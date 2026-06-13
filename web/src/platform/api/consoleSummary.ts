import { z } from "zod";

import { request } from "./http";

const consoleSummaryResponseSchema = z.object({
  version: z.string(),
  config: z.object({
    active_policy_bundle_keys: z.array(z.string()),
    policy_snapshot_hash: z.string(),
    available: z.boolean()
  }),
  policy: z.object({
    active_bundle_key_count: z.number(),
    bundle_count: z.number()
  }),
  routing: z.object({
    model_mapping_count: z.number(),
    verdict_provider_count: z.number()
  }),
  traffic: z.object({
    total: z.number(),
    blocked: z.number(),
    upstream_errors: z.number(),
    fallbacks: z.number()
  }),
  storage: z.object({
    available: z.boolean(),
    enabled: z.boolean(),
    status: z.string(),
    reason: z.string().optional(),
    depth: z.number(),
    bytes: z.number(),
    max_bytes: z.number(),
    oldest_age_seconds: z.number(),
    overflow_count: z.number()
  })
});

export type ConsoleSummaryResponse = z.infer<typeof consoleSummaryResponseSchema>;

export async function getConsoleSummary(signal?: AbortSignal): Promise<ConsoleSummaryResponse> {
  return request("/api/console/summary", consoleSummaryResponseSchema, { signal });
}
