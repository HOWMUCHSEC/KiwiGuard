import { useQuery } from "@tanstack/react-query";

import { getActiveConfig } from "platform/api/configApi";
import { queryKeys } from "platform/query/keys";

type UseDashboardOverviewDataOptions = {
  enabled?: boolean;
};

export function useDashboardOverviewData({ enabled = true }: UseDashboardOverviewDataOptions = {}) {
  const activeConfig = useQuery({
    queryKey: queryKeys.activeConfig,
    queryFn: getActiveConfig,
    retry: 1,
    enabled
  });

  return {
    snapshotHash: activeConfig.data?.policy_snapshot_hash
  };
}
