import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FormEvent, useState } from "react";

import { getActiveConfig } from "platform/api/configApi";
import { activatePolicyBundles } from "platform/api/policyActivation";
import { queryKeys } from "platform/query/keys";
import {
  createGatewayClient,
  deleteGatewayClientRouteLimit,
  patchGatewayClient,
  putGatewayClientRouteLimit,
  putGatewayRouteLimit,
  putModelMapping,
  putVerdictProvider,
  revokeGatewayClient,
  type GatewayClient,
  type GatewayClientRouteLimit,
  type GatewayRouteLimit,
  type ModelMapping,
  type VerdictProvider
} from "./routingApi";

export function useRoutingWorkflow() {
  const queryClient = useQueryClient();
  const [modelMapping, setModelMapping] = useState<ModelMapping>({
    id: "default",
    route_key: "chat",
    provider: "openai",
    model: "gpt-4o-mini",
    enabled: true
  });
  const [verdictProvider, setVerdictProvider] = useState<VerdictProvider>({
    id: "sec-model",
    name: "Vertical Security Model",
    endpoint: "http://localhost:8081/evaluate",
    mode: "inline" as const,
    enabled: true
  });
  const [createdGatewayClientKey, setCreatedGatewayClientKey] = useState("");
  const [selectedLimitClientId, setSelectedLimitClientId] = useState("");

  async function activateAccessLimitDraft(reason: string) {
    const status = await getActiveConfig();
    const response = await activatePolicyBundles({ keys: status.active_policy_bundle_keys, reason });
    if (response.notification_error) {
      throw new Error(response.notification_error);
    }
  }

  const saveModelMapping = useMutation({
    mutationFn: () => putModelMapping(modelMapping.id.trim(), { ...modelMapping, id: modelMapping.id.trim() }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.modelMappings })
      ]);
    }
  });
  const saveVerdictProvider = useMutation({
    mutationFn: () => putVerdictProvider(verdictProvider.id.trim(), { ...verdictProvider, id: verdictProvider.id.trim() }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.verdictProviders })
      ]);
    }
  });
  const createGatewayClientMutation = useMutation({
    mutationFn: (input: { name: string; notes?: string }) => createGatewayClient(input),
    onSuccess: async (response) => {
      setCreatedGatewayClientKey((key) => key || response.key);
      setSelectedLimitClientId(response.client.id);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.gatewayClients })
      ]);
    }
  });
  const saveGatewayClientMutation = useMutation({
    mutationFn: (client: GatewayClient) =>
      patchGatewayClient(client.id, {
        ...client,
        name: client.name.trim(),
        notes: client.notes?.trim() || undefined
      }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.gatewayClients })
      ]);
    }
  });
  const revokeGatewayClientMutation = useMutation({
    mutationFn: (clientID: string) => revokeGatewayClient(clientID),
    onSuccess: async (client) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.gatewayClientRouteLimits(client.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.gatewayClients })
      ]);
    }
  });
  const saveGatewayRouteLimitMutation = useMutation({
    mutationFn: async (limit: GatewayRouteLimit) => {
      const routeKey = limit.route_key.trim();
      const saved = await putGatewayRouteLimit(routeKey, { ...limit, route_key: routeKey });
      await activateAccessLimitDraft("gateway route limit update");
      return saved;
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.gatewayRouteLimits })
      ]);
    }
  });
  const saveGatewayClientRouteLimitMutation = useMutation({
    mutationFn: async (limit: GatewayClientRouteLimit) => {
      const clientID = limit.client_id.trim();
      const routeKey = limit.route_key.trim();
      const saved = await putGatewayClientRouteLimit(clientID, routeKey, { ...limit, client_id: clientID, route_key: routeKey });
      await activateAccessLimitDraft("gateway client route limit update");
      return saved;
    },
    onSuccess: async (limit) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.gatewayClientRouteLimits(limit.client_id) })
      ]);
    }
  });
  const deleteGatewayClientRouteLimitMutation = useMutation({
    mutationFn: async (input: { clientID: string; routeKey: string }) => {
      await deleteGatewayClientRouteLimit(input.clientID, input.routeKey);
      await activateAccessLimitDraft("gateway client route limit delete");
    },
    onSuccess: async (_data, input) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.gatewayClientRouteLimits(input.clientID) })
      ]);
    }
  });

  function saveModel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    saveModelMapping.mutate();
  }

  function saveProvider(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    saveVerdictProvider.mutate();
  }

  return {
    createdGatewayClientKey,
    createGatewayClientMutation,
    deleteGatewayClientRouteLimitMutation,
    gatewayAccessError:
      createGatewayClientMutation.error ??
      saveGatewayClientMutation.error ??
      revokeGatewayClientMutation.error ??
      saveGatewayRouteLimitMutation.error ??
      saveGatewayClientRouteLimitMutation.error ??
      deleteGatewayClientRouteLimitMutation.error,
    modelMapping,
    revokeGatewayClientMutation,
    saveGatewayClientMutation,
    saveGatewayClientRouteLimitMutation,
    saveGatewayRouteLimitMutation,
    saveModel,
    saveModelMapping,
    saveProvider,
    saveVerdictProvider,
    selectedLimitClientId,
    setCreatedGatewayClientKey,
    setModelMapping,
    setSelectedLimitClientId,
    setVerdictProvider,
    verdictProvider
  };
}
