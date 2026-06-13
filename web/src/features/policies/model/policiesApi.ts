import { z } from "zod";

import { request } from "platform/api/http";

export type PolicyBundle = {
  key: string;
  version: string;
  source: "built_in" | "user" | "imported";
  default_action: "allow" | "block" | "redact" | "shadow_log";
  detectors?: Array<{
    key: string;
    kind: "regex" | "email" | "phone" | "payment_card" | "secret";
    pattern?: string;
    categories?: string[];
  }>;
  rules?: Array<{
    key: string;
    enabled: boolean;
    severity: "low" | "medium" | "high" | "critical";
    action: "allow" | "block" | "redact" | "shadow_log";
    detector_keys: string[];
    scope?: {
      route_key?: string;
      provider?: string;
      model?: string;
      direction?: "input" | "output";
    };
  }>;
};

export type PolicyValidationResponse = {
  valid: boolean;
  error?: string;
  hash?: string;
};

export type PolicyActivationResponse = {
  active_keys: string[];
  hash: string;
  notification_error?: string;
  revision_number?: number;
};

export type RegexTestResponse = {
  matches: Array<{ start: number; end: number; text: string }>;
};

const policyBundleSchema: z.ZodType<PolicyBundle> = z.object({
  key: z.string(),
  version: z.string(),
  source: z.enum(["built_in", "user", "imported"]),
  default_action: z.enum(["allow", "block", "redact", "shadow_log"]),
  detectors: z
    .array(
      z.object({
        key: z.string(),
        kind: z.enum(["regex", "email", "phone", "payment_card", "secret"]),
        pattern: z.string().optional(),
        categories: z.array(z.string()).optional()
      })
    )
    .optional(),
  rules: z
    .array(
      z.object({
        key: z.string(),
        enabled: z.boolean(),
        severity: z.enum(["low", "medium", "high", "critical"]),
        action: z.enum(["allow", "block", "redact", "shadow_log"]),
        detector_keys: z.array(z.string()),
        scope: z
          .object({
            route_key: z.string().optional(),
            provider: z.string().optional(),
            model: z.string().optional(),
            direction: z.enum(["input", "output"]).optional()
          })
          .optional()
      })
    )
    .optional()
});

const policyValidationResponseSchema = z.object({
  valid: z.boolean(),
  error: z.string().optional(),
  hash: z.string().optional()
});

const regexTestResponseSchema = z.object({
  matches: z.array(z.object({ start: z.number(), end: z.number(), text: z.string() }))
});

export async function listPolicyBundles(): Promise<{ items: PolicyBundle[] }> {
  return request("/api/policies/bundles", z.object({ items: z.array(policyBundleSchema) }));
}

export async function createPolicyBundle(bundle: PolicyBundle): Promise<PolicyBundle> {
  return request("/api/policies/bundles", policyBundleSchema, {
    method: "POST",
    body: JSON.stringify(bundle)
  });
}

export async function validatePolicyBundle(bundle: PolicyBundle): Promise<PolicyValidationResponse> {
  return request("/api/policies/bundles/validate", policyValidationResponseSchema, {
    method: "POST",
    body: JSON.stringify(bundle)
  });
}

export async function testRegex(input: { pattern: string; text: string }): Promise<RegexTestResponse> {
  return request("/api/tools/regex-test", regexTestResponseSchema, {
    method: "POST",
    body: JSON.stringify(input)
  });
}
