import { z } from "zod";

import { request } from "platform/api/http";
import { type TrafficEventSummary } from "platform/api/trafficSummary";

export type { TrafficEventSummary };

export type TrafficEvent = {
  event_time: string;
  request_id: string;
  correlation_id?: string;
  route_id: string;
  provider_id: string;
  direction: "input" | "output" | "";
  action: string;
  gateway_status: number;
  upstream_status: number;
  error_type?: string;
  block_reason?: string;
  requested_model: string;
  mapped_model: string;
  latency_ms: number;
  verdict_latency_ms: number;
  detector_latency_ms: number;
  matched_span_count: number;
  fallback_triggered: boolean;
  request_hash: string;
  response_hash: string;
  request_payload: string;
  response_payload: string;
  spool_status?: string;
};

export type TrafficEventsListInput = {
  route_id?: string;
  provider_id?: string;
  direction?: "input" | "output" | "";
  status?: string;
  limit?: number;
};

export type TrafficEventsResponse = {
  items: TrafficEvent[];
  summary: TrafficEventSummary;
};

const trafficEventSchema = z.object({
  event_time: z.string(),
  request_id: z.string(),
  correlation_id: z.string().optional(),
  route_id: z.string(),
  provider_id: z.string(),
  direction: z.union([z.literal("input"), z.literal("output"), z.literal("")]),
  action: z.string(),
  gateway_status: z.number(),
  upstream_status: z.number(),
  error_type: z.string().optional(),
  block_reason: z.string().optional(),
  requested_model: z.string(),
  mapped_model: z.string(),
  latency_ms: z.number(),
  verdict_latency_ms: z.number(),
  detector_latency_ms: z.number(),
  matched_span_count: z.number(),
  fallback_triggered: z.boolean(),
  request_hash: z.string(),
  response_hash: z.string(),
  request_payload: z.string(),
  response_payload: z.string(),
  spool_status: z.string().optional()
});

const trafficEventsResponseSchema = z.object({
  items: z.array(trafficEventSchema),
  summary: z.object({
    total: z.number(),
    blocked: z.number(),
    upstream_errors: z.number(),
    fallbacks: z.number()
  })
});

export async function listTrafficEvents(input: TrafficEventsListInput): Promise<TrafficEventsResponse> {
  const params = new URLSearchParams();
  if (input.route_id) params.set("route_id", input.route_id);
  if (input.provider_id) params.set("provider_id", input.provider_id);
  if (input.direction) params.set("direction", input.direction);
  if (input.status) params.set("status", input.status);
  if (input.limit) params.set("limit", String(input.limit));
  const suffix = params.toString();
  return request(`/api/traffic/events${suffix ? `?${suffix}` : ""}`, trafficEventsResponseSchema);
}
