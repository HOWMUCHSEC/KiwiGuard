import { useMemo } from "react";

import { consoleMetricDefinitions, consoleMetricDefinition, type ConsoleDestination, type ConsoleMetricKey } from "console/routes";
import type { ConsolePostureSummary } from "shared/types/console";
import { MetricTile } from "shared/ui/MetricTile";
import type { LucideIcon } from "lucide-react";

type MetricSummaryCard = {
  key: ConsoleMetricKey;
  label: string;
  value: string;
  icon: LucideIcon;
};

type ConsoleSummaryProps = {
  onNavigate: (destination: ConsoleDestination) => void;
  summary: ConsolePostureSummary;
  t: (key: string, values?: Record<string, string | number>) => string;
};

export function ConsoleSummary({ onNavigate, summary, t }: ConsoleSummaryProps) {
  const unknownLabel = t("common.unknown");
  const summaryCards = useMemo<MetricSummaryCard[]>(
    () => consoleMetricDefinitions.map((definition) => ({
      key: definition.key,
      label: t(definition.labelKey),
      value: definition.selectValue(summary, unknownLabel),
      icon: definition.icon
    })),
    [summary, t, unknownLabel]
  );

  return (
    <section className="kg-summary kg-summary--console" aria-label={t("summary.aria")}>
      {summaryCards.map((card) => (
        <MetricTile
          key={card.key}
          label={card.label}
          value={card.value}
          icon={card.icon}
          ariaLabel={t("nav.metricCard", { label: card.label })}
          onClick={() => onNavigate(consoleMetricDefinition(card.key).destination)}
        />
      ))}
    </section>
  );
}
