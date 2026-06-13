import { FormEvent } from "react";
import { Route, Save } from "lucide-react";
import { Button, InlineNotification, TextInput, Tile, Toggle } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { PanelHeader } from "shared/ui/PanelHeader";
import type { PageHeading } from "shared/ui/PageHeading";
import type { ModelMapping } from "../model/routingApi";

type RoutingMappingSectionProps = {
  heading: PageHeading;
  mappingItems: ModelMapping[];
  modelMapping: ModelMapping;
  saveError?: unknown;
  saveIsError: boolean;
  saveIsPending: boolean;
  saveIsSuccess: boolean;
  onModelMappingChange: (mapping: ModelMapping) => void;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
};

export function RoutingMappingSection({
  heading,
  mappingItems,
  modelMapping,
  saveError,
  saveIsError,
  saveIsPending,
  saveIsSuccess,
  onModelMappingChange,
  onSave
}: RoutingMappingSectionProps) {
  const { t } = useI18n();

  return (
    <Tile className="kg-panel">
      <PanelHeader icon={<Route aria-hidden="true" />} kicker={heading.kicker} title={heading.title} />
      <form className="config-form" onSubmit={onSave}>
        <div className="form-grid">
          <TextInput id="mapping-id" labelText={t("routing.id")} value={modelMapping.id} onChange={(event) => onModelMappingChange({ ...modelMapping, id: event.target.value })} />
          <TextInput id="mapping-route" labelText={t("routing.route")} value={modelMapping.route_key} onChange={(event) => onModelMappingChange({ ...modelMapping, route_key: event.target.value })} />
          <TextInput id="mapping-provider" labelText={t("routing.provider")} value={modelMapping.provider} onChange={(event) => onModelMappingChange({ ...modelMapping, provider: event.target.value })} />
          <TextInput id="mapping-model" labelText={t("routing.model")} value={modelMapping.model} onChange={(event) => onModelMappingChange({ ...modelMapping, model: event.target.value })} />
        </div>
        <Toggle id="mapping-enabled" labelText={t("routing.enabled")} toggled={modelMapping.enabled} onToggle={(enabled) => onModelMappingChange({ ...modelMapping, enabled })} />
        <Button type="submit" renderIcon={Save} disabled={saveIsPending}>
          {saveIsPending ? t("routing.saving") : t("routing.save")}
        </Button>
      </form>
      {saveIsError ? (
        <InlineNotification kind="error" lowContrast hideCloseButton title={t("routing.issue")} subtitle={saveError instanceof Error ? saveError.message : t("routing.saveFailed")} />
      ) : null}
      {saveIsSuccess ? <InlineNotification kind="success" lowContrast hideCloseButton title={t("routing.update")} subtitle={t("routing.saved")} /> : null}
      <CompactMappingList mappings={mappingItems} />
    </Tile>
  );
}

function CompactMappingList({ mappings }: { mappings: ModelMapping[] }) {
  const { t } = useI18n();

  return (
    <div className="compact-list">
      {mappings.length === 0 ? (
        <p className="kg-muted">{t("routing.empty")}</p>
      ) : (
        mappings.map((mapping) => (
          <div className="list-row" key={mapping.id}>
            <span>{mapping.route_key}</span>
            <strong>{mapping.provider}</strong>
            <code>{mapping.enabled ? mapping.model : `${mapping.model} ${t("common.off")}`}</code>
          </div>
        ))
      )}
    </div>
  );
}
