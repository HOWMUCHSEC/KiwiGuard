import { Activity, ListChecks } from "lucide-react";
import { Button, Checkbox, InlineNotification, Tag, TextArea, Tile } from "@carbon/react";

import type { PolicyBundle } from "../model/policiesApi";
import type { SpoolStatusResponse } from "platform/api/spool";
import { useI18n } from "platform/i18n";
import { PanelHeader } from "shared/ui/PanelHeader";
import type { PageHeading } from "shared/ui/PageHeading";
import { SpoolStatusCard } from "shared/ui/SpoolStatusCard";
import type { MessageKind } from "shared/ui/status";

type PolicyActivationPanelProps = {
  activeKeys: string[];
  activationMessage: string;
  activationMessageKind: MessageKind;
  activationReason: string;
  heading: PageHeading;
  isActivating: boolean;
  isSpoolError: boolean;
  isSpoolLoading: boolean;
  isValidating: boolean;
  policyItems: PolicyBundle[];
  selectedPolicyKeys: string[];
  snapshotHash: string;
  spool?: SpoolStatusResponse;
  spoolTitle: string;
  version: string;
  onActivate: () => void;
  onReasonChange: (value: string) => void;
  onSelectionChange: (key: string, checked: boolean) => void;
  onValidate: () => void;
};

export function PolicyActivationPanel({
  activeKeys,
  activationMessage,
  activationMessageKind,
  activationReason,
  heading,
  isActivating,
  isSpoolError,
  isSpoolLoading,
  isValidating,
  policyItems,
  selectedPolicyKeys,
  snapshotHash,
  spool,
  spoolTitle,
  version,
  onActivate,
  onReasonChange,
  onSelectionChange,
  onValidate
}: PolicyActivationPanelProps) {
  const { t } = useI18n();

  return (
    <Tile className="kg-panel">
      <PanelHeader icon={<Activity aria-hidden="true" />} kicker={heading.kicker} title={heading.title} />
      <div className="kg-panel-body kg-stack">
        <dl className="stacked-list">
          <div>
            <dt>{t("policy.version")}</dt>
            <dd>{version}</dd>
          </div>
          <div>
            <dt>{t("runtime.snapshot")}</dt>
            <dd>{snapshotHash}</dd>
          </div>
          <div>
            <dt>{t("runtime.activeKeys")}</dt>
            <dd>{activeKeys.length ? activeKeys.join(", ") : t("common.none")}</dd>
          </div>
        </dl>
        <SpoolStatusCard isError={isSpoolError} isLoading={isSpoolLoading} spool={spool} t={t} title={spoolTitle} />
      </div>
      <div className="activation-panel">
        <div className="checkbox-list" aria-label={t("runtime.bundleSelectionAria")}>
          {policyItems.length === 0 ? (
            <p className="kg-muted">{t("runtime.emptyBundles")}</p>
          ) : (
            policyItems.map((bundle) => (
              <Checkbox
                id={`policy-${bundle.key}`}
                key={bundle.key}
                labelText={
                  <span>
                    {bundle.key}
                    {activeKeys.includes(bundle.key) ? <Tag type="green">{t("runtime.active")}</Tag> : null}
                  </span>
                }
                checked={selectedPolicyKeys.includes(bundle.key)}
                onChange={(_, { checked }) => onSelectionChange(bundle.key, checked)}
              />
            ))
          )}
        </div>
        <TextArea
          id="activation-reason"
          labelText={t("runtime.reason")}
          value={activationReason}
          onChange={(event) => onReasonChange(event.target.value)}
          rows={3}
          placeholder={t("runtime.reasonPlaceholder")}
        />
        <div className="kg-inline-actions">
          <Button kind="secondary" type="button" renderIcon={ListChecks} onClick={onValidate} disabled={isValidating}>
            {isValidating ? t("runtime.validating") : t("runtime.validateSelected")}
          </Button>
          <Button type="button" renderIcon={Activity} onClick={onActivate} disabled={isActivating}>
            {isActivating ? t("runtime.activating") : t("runtime.activateSelected")}
          </Button>
        </div>
        {activationMessage ? (
          <InlineNotification
            kind={activationMessageKind}
            lowContrast
            hideCloseButton
            title={activationMessageKind === "error" ? t("runtime.issue") : t("runtime.update")}
            subtitle={activationMessage}
          />
        ) : null}
      </div>
    </Tile>
  );
}
