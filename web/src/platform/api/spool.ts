import { z } from "zod";
import type { SpoolStatusSnapshot } from "shared/types/spool";
import { request } from "./http";

export const spoolStatusResponseSchema = z.object({
  enabled: z.boolean(),
  status: z.string(),
  reason: z.string().optional(),
  depth: z.number(),
  bytes: z.number(),
  max_bytes: z.number(),
  oldest_age_seconds: z.number(),
  overflow_count: z.number()
});

export type SpoolStatusResponse = z.infer<typeof spoolStatusResponseSchema> & SpoolStatusSnapshot;

export async function getTrafficSpoolStatus(): Promise<SpoolStatusResponse> {
  return request("/api/storage/event-spool", spoolStatusResponseSchema);
}
