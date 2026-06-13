export function shortHash(value: string, emptyLabel = "none") {
  if (!value) return emptyLabel;
  return value.slice(0, 10);
}

export function formatBytes(value: number) {
  if (value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let scaled = value;
  let unit = 0;
  while (scaled >= 1024 && unit < units.length - 1) {
    scaled /= 1024;
    unit++;
  }
  return `${scaled >= 10 || unit === 0 ? scaled.toFixed(0) : scaled.toFixed(1)} ${units[unit]}`;
}

export function formatAge(seconds: number, emptyLabel = "none") {
  if (seconds <= 0) return emptyLabel;
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  return `${Math.round(seconds / 3600)}h`;
}

export function formatPayload(value: string, emptyLabel = "Raw capture disabled or unavailable.") {
  if (!value) return emptyLabel;
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}
