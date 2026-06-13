import { useQuery } from "@tanstack/react-query";

import { getActiveConfig, getHealth } from "platform/api/configApi";
import { getTrafficSpoolStatus } from "platform/api/spool";
import { queryKeys } from "platform/query/keys";

type UseStoragePageDataOptions = {
  enabled?: boolean;
};

export function useStoragePageData({ enabled = true }: UseStoragePageDataOptions = {}) {
  const health = useQuery({
    queryKey: queryKeys.health,
    queryFn: getHealth,
    retry: 1,
    enabled
  });
  const activeConfig = useQuery({
    queryKey: queryKeys.activeConfig,
    queryFn: getActiveConfig,
    retry: 1,
    enabled
  });
  const spoolStatus = useQuery({
    queryKey: queryKeys.trafficSpool,
    queryFn: getTrafficSpoolStatus,
    retry: 1,
    refetchInterval: 5000,
    enabled
  });

  return {
    activeConfig,
    health,
    spool: spoolStatus.data,
    spoolStatus,
    version: health.data?.version ?? "unknown",
    snapshotHash: activeConfig.data?.policy_snapshot_hash
  };
}
