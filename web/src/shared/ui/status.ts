export type TagTypeName = "red" | "green" | "blue" | "purple" | "gray" | "cool-gray" | "warm-gray" | "magenta" | "cyan" | "teal" | "high-contrast" | "outline";
export type MessageKind = "error" | "success";

export function healthLabelKey(status: string) {
  if (status === "degraded") return "health.degraded";
  if (status === "unhealthy") return "health.unhealthy";
  if (status === "ok") return "health.ok";
  return "health.unknown";
}

export function healthTagType(status: string): TagTypeName {
  if (status === "ok") return "green";
  if (status === "degraded") return "warm-gray";
  if (status === "unhealthy") return "red";
  return "gray";
}

export function spoolLabel(status: string | undefined) {
  if (status === "ok") return "spool.healthy";
  if (status === "backlogged") return "spool.backlogged";
  if (status === "degraded") return "spool.degraded";
  if (status === "disabled") return "spool.disabled";
  if (status === "spooled") return "spool.spooled";
  if (status === "replayed") return "spool.replayed";
  if (!status) return "spool.direct";
  return status;
}

export function spoolState(status: string | undefined) {
  if (status === "spooled" || status === "replayed" || status === "backlogged" || status === "degraded" || status === "disabled") return status;
  return "direct";
}

export function spoolTagType(status: string | undefined): TagTypeName {
  if (status === "replayed" || status === "ok") return "green";
  if (status === "spooled" || status === "backlogged") return "warm-gray";
  if (status === "degraded") return "red";
  if (status === "disabled") return "gray";
  return "cool-gray";
}

export function actionTagType(action: string | undefined): TagTypeName {
  if (action === "block") return "red";
  if (action === "redact") return "purple";
  if (action === "allow") return "green";
  return "gray";
}

export function messageKind(message: string): MessageKind {
  const lower = message.toLowerCase();
  return lower.includes("failed") || lower.includes("invalid") || lower.includes("not valid") || lower.includes("select") ? "error" : "success";
}

export function messageClassName(message: string) {
  return messageKind(message) === "error" ? "error-text" : "success-text";
}
