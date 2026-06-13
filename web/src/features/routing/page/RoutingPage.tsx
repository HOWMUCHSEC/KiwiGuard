import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "platform/query/keys";
import type { PageHeading } from "shared/ui/PageHeading";
import type { GatewayClient, GatewayClientRouteLimit, GatewayRouteLimit } from "../model/routingApi";
import { ConfigurationPanels } from "../ui/ConfigurationPanels";
import {
  listGatewayClientRouteLimits,
  listGatewayClients,
  listGatewayRouteLimits,
  listModelMappings,
  listVerdictProviders
} from "../model/routingApi";
import { useRoutingWorkflow } from "../model/useRoutingWorkflow";
import "./routing.css";

export type RoutingPageProps = {
  activeSection?: "route-mapping" | "access-limits" | "providers";
  accessHeading: PageHeading;
  mappingHeading: PageHeading;
  providerHeading: PageHeading;
};

export function RoutingPage({ accessHeading, activeSection, mappingHeading, providerHeading }: RoutingPageProps) {
  const routingWorkflow = useRoutingWorkflow();
  const mappings = useQuery({
    queryKey: queryKeys.modelMappings,
    queryFn: listModelMappings,
    retry: 1
  });
  const providers = useQuery({
    queryKey: queryKeys.verdictProviders,
    queryFn: listVerdictProviders,
    retry: 1
  });
  const gatewayClients = useQuery({
    queryKey: queryKeys.gatewayClients,
    queryFn: listGatewayClients,
    retry: 1
  });
  const gatewayRouteLimits = useQuery({
    queryKey: queryKeys.gatewayRouteLimits,
    queryFn: listGatewayRouteLimits,
    retry: 1
  });
  const gatewayClientRouteLimits = useQuery({
    queryKey: queryKeys.gatewayClientRouteLimits(routingWorkflow.selectedLimitClientId),
    queryFn: () => listGatewayClientRouteLimits(routingWorkflow.selectedLimitClientId),
    enabled: routingWorkflow.selectedLimitClientId.trim().length > 0,
    retry: 1
  });

  return (
    <ConfigurationPanels
      activeSection={activeSection}
      accessError={routingWorkflow.gatewayAccessError}
      accessHeading={accessHeading}
      clientRouteLimitItems={gatewayClientRouteLimits.data?.items ?? []}
      createdClientKey={routingWorkflow.createdGatewayClientKey}
      defaultRouteLimitItems={gatewayRouteLimits.data?.items ?? []}
      gatewayClients={gatewayClients.data?.items ?? []}
      isCreatingGatewayClient={routingWorkflow.createGatewayClientMutation.isPending}
      isDeletingClientRouteLimit={routingWorkflow.deleteGatewayClientRouteLimitMutation.isPending}
      isRevokingGatewayClient={routingWorkflow.revokeGatewayClientMutation.isPending}
      isSavingClientRouteLimit={routingWorkflow.saveGatewayClientRouteLimitMutation.isPending}
      isSavingGatewayClient={routingWorkflow.saveGatewayClientMutation.isPending}
      isSavingRouteLimit={routingWorkflow.saveGatewayRouteLimitMutation.isPending}
      mappingItems={mappings.data?.items ?? []}
      modelMapping={routingWorkflow.modelMapping}
      mappingHeading={mappingHeading}
      providerItems={providers.data?.items ?? []}
      providerHeading={providerHeading}
      saveMappingError={routingWorkflow.saveModelMapping.error}
      saveMappingIsError={routingWorkflow.saveModelMapping.isError}
      saveMappingIsPending={routingWorkflow.saveModelMapping.isPending}
      saveMappingIsSuccess={routingWorkflow.saveModelMapping.isSuccess}
      saveProviderError={routingWorkflow.saveVerdictProvider.error}
      saveProviderIsError={routingWorkflow.saveVerdictProvider.isError}
      saveProviderIsPending={routingWorkflow.saveVerdictProvider.isPending}
      saveProviderIsSuccess={routingWorkflow.saveVerdictProvider.isSuccess}
      selectedLimitClientId={routingWorkflow.selectedLimitClientId}
      verdictProvider={routingWorkflow.verdictProvider}
      onClearCreatedClientKey={() => routingWorkflow.setCreatedGatewayClientKey("")}
      onCreateGatewayClient={(input) => routingWorkflow.createGatewayClientMutation.mutate(input)}
      onDeleteClientRouteLimit={(clientID, routeKey) => routingWorkflow.deleteGatewayClientRouteLimitMutation.mutate({ clientID, routeKey })}
      onModelMappingChange={routingWorkflow.setModelMapping}
      onPatchGatewayClient={(client: GatewayClient) => routingWorkflow.saveGatewayClientMutation.mutate(client)}
      onRevokeGatewayClient={(clientID: string) => routingWorkflow.revokeGatewayClientMutation.mutate(clientID)}
      onSaveClientRouteLimit={(limit: GatewayClientRouteLimit) => routingWorkflow.saveGatewayClientRouteLimitMutation.mutate(limit)}
      onSaveDefaultRouteLimit={(limit: GatewayRouteLimit) => routingWorkflow.saveGatewayRouteLimitMutation.mutate(limit)}
      onSaveMapping={routingWorkflow.saveModel}
      onSaveProvider={routingWorkflow.saveProvider}
      onSelectedLimitClientChange={routingWorkflow.setSelectedLimitClientId}
      onVerdictProviderChange={routingWorkflow.setVerdictProvider}
    />
  );
}
