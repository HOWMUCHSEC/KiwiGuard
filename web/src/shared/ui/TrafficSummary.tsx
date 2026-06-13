import type { TrafficSummarySnapshot } from "shared/types/traffic";

type TrafficSummaryProps = {
  labels: {
    aria: string;
    blocked: string;
    fallbacks: string;
    total: string;
    upstreamErrors: string;
  };
  summary: TrafficSummarySnapshot;
};

export function TrafficSummary({ labels, summary }: TrafficSummaryProps) {
  return (
    <div className="traffic-summary" aria-label={labels.aria}>
      <span>
        {labels.total} <strong>{summary.total}</strong>
      </span>
      <span>
        {labels.blocked} <strong>{summary.blocked}</strong>
      </span>
      <span>
        {labels.upstreamErrors} <strong>{summary.upstream_errors}</strong>
      </span>
      <span>
        {labels.fallbacks} <strong>{summary.fallbacks}</strong>
      </span>
    </div>
  );
}
