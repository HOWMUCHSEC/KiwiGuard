import { FormEvent } from "react";
import { KeyRound } from "lucide-react";
import { Tile } from "@carbon/react";

import type { GatewayClient, GatewayClientRouteLimit, GatewayRouteLimit, ModelMapping, VerdictProvider } from "../model/routingApi";
import { GatewayAccessLimitsPanel } from "./GatewayAccessLimitsPanel";
import { RoutingMappingSection } from "./RoutingMappingSection";
import { RoutingProviderSection } from "./RoutingProviderSection";
import { PanelHeader } from "shared/ui/PanelHeader";
import type { PageHeading } from "shared/ui/PageHeading";

type ConfigurationPanelsProps = {
  activeSection?: "route-mapping" | "access-limits" | "providers";
  accessError?: unknown;
  accessHeading: PageHeading;
  clientRouteLimitItems: GatewayClientRouteLimit[];
  createdClientKey?: string;
  defaultRouteLimitItems: GatewayRouteLimit[];
  gatewayClients: GatewayClient[];
  isCreatingGatewayClient: boolean;
  isDeletingClientRouteLimit: boolean;
  isRevokingGatewayClient: boolean;
  isSavingClientRouteLimit: boolean;
  isSavingGatewayClient: boolean;
  isSavingRouteLimit: boolean;
  mappingItems: ModelMapping[];
  modelMapping: ModelMapping;
  mappingHeading: PageHeading;
  providerItems: VerdictProvider[];
  providerHeading: PageHeading;
  selectedLimitClientId: string;
  saveMappingError?: unknown;
  saveMappingIsError: boolean;
  saveMappingIsPending: boolean;
  saveMappingIsSuccess: boolean;
  saveProviderError?: unknown;
  saveProviderIsError: boolean;
  saveProviderIsPending: boolean;
  saveProviderIsSuccess: boolean;
  verdictProvider: VerdictProvider;
  onClearCreatedClientKey: () => void;
  onCreateGatewayClient: (input: { name: string; notes?: string }) => void;
  onDeleteClientRouteLimit: (clientID: string, routeKey: string) => void;
  onModelMappingChange: (mapping: ModelMapping) => void;
  onPatchGatewayClient: (client: GatewayClient) => void;
  onRevokeGatewayClient: (clientID: string) => void;
  onSaveClientRouteLimit: (limit: GatewayClientRouteLimit) => void;
  onSaveDefaultRouteLimit: (limit: GatewayRouteLimit) => void;
  onSaveMapping: (event: FormEvent<HTMLFormElement>) => void;
  onSaveProvider: (event: FormEvent<HTMLFormElement>) => void;
  onSelectedLimitClientChange: (clientId: string) => void;
  onVerdictProviderChange: (provider: VerdictProvider) => void;
};

export function ConfigurationPanels({
  activeSection,
  accessError,
  accessHeading,
  clientRouteLimitItems,
  createdClientKey,
  defaultRouteLimitItems,
  gatewayClients,
  isCreatingGatewayClient,
  isDeletingClientRouteLimit,
  isRevokingGatewayClient,
  isSavingClientRouteLimit,
  isSavingGatewayClient,
  isSavingRouteLimit,
  mappingItems,
  modelMapping,
  mappingHeading,
  providerItems,
  providerHeading,
  selectedLimitClientId,
  saveMappingError,
  saveMappingIsError,
  saveMappingIsPending,
  saveMappingIsSuccess,
  saveProviderError,
  saveProviderIsError,
  saveProviderIsPending,
  saveProviderIsSuccess,
  verdictProvider,
  onClearCreatedClientKey,
  onCreateGatewayClient,
  onDeleteClientRouteLimit,
  onModelMappingChange,
  onPatchGatewayClient,
  onRevokeGatewayClient,
  onSaveClientRouteLimit,
  onSaveDefaultRouteLimit,
  onSaveMapping,
  onSaveProvider,
  onSelectedLimitClientChange,
  onVerdictProviderChange
}: ConfigurationPanelsProps) {
  const sections = [
    !activeSection || activeSection === "route-mapping"
      ? (
        <RoutingMappingSection
          key="route-mapping"
          heading={mappingHeading}
          mappingItems={mappingItems}
          modelMapping={modelMapping}
          saveError={saveMappingError}
          saveIsError={saveMappingIsError}
          saveIsPending={saveMappingIsPending}
          saveIsSuccess={saveMappingIsSuccess}
          onModelMappingChange={onModelMappingChange}
          onSave={onSaveMapping}
        />
      )
      : null,
    !activeSection || activeSection === "access-limits"
      ? (
        <Tile className="kg-panel" key="access-limits">
          <PanelHeader icon={<KeyRound aria-hidden="true" />} kicker={accessHeading.kicker} title={accessHeading.title} />
          <GatewayAccessLimitsPanel
            clients={gatewayClients}
            routeLimits={defaultRouteLimitItems}
            clientRouteLimits={clientRouteLimitItems}
            selectedClientID={selectedLimitClientId}
            createdKey={createdClientKey}
            error={accessError}
            isCreatingClient={isCreatingGatewayClient}
            isSavingClient={isSavingGatewayClient}
            isRevokingClient={isRevokingGatewayClient}
            isSavingRouteLimit={isSavingRouteLimit}
            isSavingClientLimit={isSavingClientRouteLimit}
            isDeletingClientLimit={isDeletingClientRouteLimit}
            onClearCreatedKey={onClearCreatedClientKey}
            onSelectedClientChange={onSelectedLimitClientChange}
            onCreateClient={onCreateGatewayClient}
            onPatchClient={onPatchGatewayClient}
            onRevokeClient={onRevokeGatewayClient}
            onSaveRouteLimit={onSaveDefaultRouteLimit}
            onSaveClientLimit={onSaveClientRouteLimit}
            onDeleteClientLimit={onDeleteClientRouteLimit}
          />
        </Tile>
      )
      : null,
    !activeSection || activeSection === "providers"
      ? (
        <RoutingProviderSection
          key="providers"
          heading={providerHeading}
          providerItems={providerItems}
          verdictProvider={verdictProvider}
          saveError={saveProviderError}
          saveIsError={saveProviderIsError}
          saveIsPending={saveProviderIsPending}
          saveIsSuccess={saveProviderIsSuccess}
          onSave={onSaveProvider}
          onVerdictProviderChange={onVerdictProviderChange}
        />
      )
      : null
  ].filter(Boolean);

  return <div className="kg-routing-section-stack">{sections}</div>;
}
