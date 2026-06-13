export type SpoolStatusSnapshot = {
  status?: string;
  reason?: string;
  depth: number;
  bytes: number;
  max_bytes: number;
  oldest_age_seconds: number;
  overflow_count: number;
};
