import type { PolicyBundle } from "./policiesApi";

export const starterPolicyBundle = {
  key: "user-pii",
  version: "1.0.0",
  source: "user",
  default_action: "allow",
  detectors: [
    {
      key: "company-email",
      kind: "regex",
      pattern: "[a-zA-Z0-9._%+-]+@example\\.com",
      categories: ["email"]
    }
  ],
  rules: [
    {
      key: "redact-company-email",
      enabled: true,
      severity: "medium",
      action: "redact",
      detector_keys: ["company-email"],
      scope: {
        direction: "input"
      }
    }
  ]
} satisfies PolicyBundle;

export const starterPolicyBundleJSON = JSON.stringify(starterPolicyBundle, null, 2);
