import { useQuery } from "@tanstack/react-query";

import { emptyTrafficEventSummary } from "platform/api/trafficSummary";
import { queryKeys } from "platform/query/keys";
import { listTrafficEvents, type TrafficEventsListInput } from "./trafficApi";

export function useTrafficEventsPageData(filters: TrafficEventsListInput) {
  const traffic = useQuery({
    queryKey: queryKeys.trafficEvents(filters),
    queryFn: () => listTrafficEvents(filters),
    retry: 1
  });

  return {
    traffic,
    events: traffic.data?.items ?? [],
    summary: traffic.data?.summary ?? emptyTrafficEventSummary
  };
}
