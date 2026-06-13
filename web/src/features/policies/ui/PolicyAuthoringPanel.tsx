import { FormEvent } from "react";
import { ListChecks, Save, ShieldCheck } from "lucide-react";
import { Button, InlineNotification, TextArea, Tile } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { PanelHeader } from "shared/ui/PanelHeader";
import type { MessageKind } from "shared/ui/status";

type PolicyAuthoringPanelProps = {
  message: string;
  messageKind: MessageKind;
  policyBundleJSON: string;
  isSaving: boolean;
  isValidating: boolean;
  onChange: (value: string) => void;
  onSave: () => void;
  onValidate: (event: FormEvent<HTMLFormElement>) => void;
};

export function PolicyAuthoringPanel({ message, messageKind, policyBundleJSON, isSaving, isValidating, onChange, onSave, onValidate }: PolicyAuthoringPanelProps) {
  const { t } = useI18n();

  return (
    <Tile className="kg-panel">
      <PanelHeader icon={<ShieldCheck aria-hidden="true" />} kicker={t("authoring.kicker")} title={t("authoring.title")} />
      <form className="config-form" onSubmit={onValidate}>
        <TextArea
          id="policy-bundle-json"
          className="code-input"
          labelText={t("authoring.document")}
          value={policyBundleJSON}
          onChange={(event) => onChange(event.target.value)}
          rows={18}
          spellCheck={false}
        />
        <div className="kg-inline-actions">
          <Button kind="secondary" type="submit" renderIcon={ListChecks} disabled={isValidating}>
            {isValidating ? t("runtime.validating") : t("authoring.validate")}
          </Button>
          <Button type="button" renderIcon={Save} onClick={onSave} disabled={isSaving}>
            {isSaving ? t("routing.saving") : t("authoring.save")}
          </Button>
        </div>
      </form>
      {message ? (
        <InlineNotification kind={messageKind} lowContrast hideCloseButton title={messageKind === "error" ? t("authoring.issue") : t("authoring.update")} subtitle={message} />
      ) : null}
    </Tile>
  );
}
