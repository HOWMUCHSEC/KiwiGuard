import { FormEvent } from "react";
import { FlaskConical } from "lucide-react";
import { Button, InlineNotification, Tag, TextArea, TextInput, Tile } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { PanelHeader } from "shared/ui/PanelHeader";

type RegexLabPanelProps = {
  isError: boolean;
  isPending: boolean;
  isSuccess: boolean;
  matches: Array<{ start: number; end: number; text: string }>;
  pattern: string;
  sampleText: string;
  onPatternChange: (value: string) => void;
  onRun: (event: FormEvent<HTMLFormElement>) => void;
  onSampleTextChange: (value: string) => void;
};

export function RegexLabPanel({ isError, isPending, isSuccess, matches, pattern, sampleText, onPatternChange, onRun, onSampleTextChange }: RegexLabPanelProps) {
  const { t } = useI18n();

  return (
    <Tile className="kg-panel">
      <PanelHeader icon={<FlaskConical aria-hidden="true" />} kicker={t("regex.kicker")} title={t("regex.title")} />
      <form className="regex-form" onSubmit={onRun}>
        <TextInput id="regex-pattern" labelText={t("regex.pattern")} value={pattern} onChange={(event) => onPatternChange(event.target.value)} spellCheck={false} />
        <TextArea id="regex-sample" labelText={t("regex.sample")} value={sampleText} onChange={(event) => onSampleTextChange(event.target.value)} rows={4} />
        <Button type="submit" renderIcon={FlaskConical} disabled={isPending}>
          {isPending ? t("regex.running") : t("regex.run")}
        </Button>
      </form>
      <div className="match-strip">
        {isError ? <InlineNotification kind="error" lowContrast hideCloseButton title={t("regex.issue")} subtitle={t("regex.failed")} /> : null}
        {matches.length === 0 && isSuccess ? <p className="kg-muted">{t("regex.empty")}</p> : null}
        {matches.map((match) => (
          <Tag type="warm-gray" key={`${match.start}-${match.end}-${match.text}`}>
            {match.start}-{match.end}: {match.text}
          </Tag>
        ))}
      </div>
    </Tile>
  );
}
