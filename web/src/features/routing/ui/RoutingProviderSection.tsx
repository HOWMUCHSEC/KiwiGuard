import { FormEvent } from "react";
import { Database, Save } from "lucide-react";
import { Button, InlineNotification, TextInput, Tile, Toggle } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { PanelHeader } from "shared/ui/PanelHeader";
import type { PageHeading } from "shared/ui/PageHeading";
import type { VerdictProvider } from "../model/routingApi";

type RoutingProviderSectionProps = {
  heading: PageHeading;
  providerItems: VerdictProvider[];
  verdictProvider: VerdictProvider;
  saveError?: unknown;
  saveIsError: boolean;
  saveIsPending: boolean;
  saveIsSuccess: boolean;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
  onVerdictProviderChange: (provider: VerdictProvider) => void;
};

export function RoutingProviderSection({
  heading,
  providerItems,
  verdictProvider,
  saveError,
  saveIsError,
  saveIsPending,
  saveIsSuccess,
  onSave,
  onVerdictProviderChange
}: RoutingProviderSectionProps) {
  const { t } = useI18n();

  return (
    <Tile className="kg-panel">
      <PanelHeader icon={<Database aria-hidden="true" />} kicker={heading.kicker} title={heading.title} />
      <form className="config-form" onSubmit={onSave}>
        <div className="form-grid">
          <TextInput id="provider-id" labelText={t("routing.id")} value={verdictProvider.id} onChange={(event) => onVerdictProviderChange({ ...verdictProvider, id: event.target.value })} />
          <TextInput id="provider-name" labelText={t("provider.name")} value={verdictProvider.name} onChange={(event) => onVerdictProviderChange({ ...verdictProvider, name: event.target.value })} />
          <TextInput
            id="provider-endpoint"
            className="wide-field"
            labelText={t("provider.endpoint")}
            value={verdictProvider.endpoint}
            onChange={(event) => onVerdictProviderChange({ ...verdictProvider, endpoint: event.target.value })}
          />
          <TextInput
            id="provider-credential-ref"
            labelText={t("provider.credentialRef")}
            value={verdictProvider.credential_ref ?? ""}
            onChange={(event) => onVerdictProviderChange({ ...verdictProvider, credential_ref: event.target.value })}
          />
          <TextInput id="provider-mode" labelText={t("provider.mode")} value={verdictProvider.mode} readOnly />
        </div>
        <Toggle id="provider-enabled" labelText={t("routing.enabled")} toggled={verdictProvider.enabled} onToggle={(enabled) => onVerdictProviderChange({ ...verdictProvider, enabled })} />
        <Button type="submit" renderIcon={Save} disabled={saveIsPending}>
          {saveIsPending ? t("routing.saving") : t("provider.save")}
        </Button>
      </form>
      {saveIsError ? (
        <InlineNotification kind="error" lowContrast hideCloseButton title={t("provider.issue")} subtitle={saveError instanceof Error ? saveError.message : t("provider.saveFailed")} />
      ) : null}
      {saveIsSuccess ? <InlineNotification kind="success" lowContrast hideCloseButton title={t("provider.update")} subtitle={t("provider.saved")} /> : null}
      <CompactProviderList providers={providerItems} />
    </Tile>
  );
}

function CompactProviderList({ providers }: { providers: VerdictProvider[] }) {
  const { t } = useI18n();

  return (
    <div className="compact-list">
      {providers.length === 0 ? (
        <p className="kg-muted">{t("provider.empty")}</p>
      ) : (
        providers.map((provider) => (
          <div className="list-row" key={provider.id}>
            <span>{provider.name}</span>
            <strong>{provider.mode}</strong>
            <code>{provider.enabled ? t("common.enabled") : t("common.disabled")}</code>
          </div>
        ))
      )}
    </div>
  );
}
