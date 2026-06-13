import { z } from "zod";

import { request } from "./http";

export type PolicyActivationResponse = {
  active_keys: string[];
  hash: string;
  notification_error?: string;
  revision_number?: number;
};

const policyActivationResponseSchema = z.object({
  active_keys: z.array(z.string()),
  hash: z.string(),
  notification_error: z.string().optional(),
  revision_number: z.number().optional()
});

export async function activatePolicyBundles(input: { keys: string[]; reason?: string }): Promise<PolicyActivationResponse> {
  return request("/api/policies/bundles/activate", policyActivationResponseSchema, {
    method: "POST",
    body: JSON.stringify(input)
  });
}
