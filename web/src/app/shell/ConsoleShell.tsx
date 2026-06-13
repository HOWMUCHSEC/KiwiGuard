import type { ReactNode } from "react";
import { Column, Content, Grid, Theme } from "@carbon/react";

import type { ConsoleDestination } from "console/routes";
import type { ConsolePostureSummary } from "shared/types/console";
import { ConsoleNavigation } from "console/ConsoleNavigation";
import { ConsoleSummary } from "./ConsoleSummary";

type ConsoleShellProps = {
  activeDestination: ConsoleDestination;
  children: ReactNode;
  onNavigate: (destination: ConsoleDestination) => void;
  summary: ConsolePostureSummary;
  t: (key: string, values?: Record<string, string | number>) => string;
};

export function ConsoleShell({ activeDestination, children, onNavigate, summary, t }: ConsoleShellProps) {
  return (
    <Theme theme="g100">
      <Content className="kg-console">
        <Grid fullWidth narrow className="kg-grid">
          <Column sm={4} md={8} lg={16}>
            <section className="kg-console-top">
              <ConsoleSummary onNavigate={onNavigate} summary={summary} t={t} />
              <ConsoleNavigation active={activeDestination} onNavigate={onNavigate} />
            </section>
          </Column>
          <Column sm={4} md={8} lg={16}>
            <section className="kg-workspace-grid kg-console-workspace">{children}</section>
          </Column>
        </Grid>
      </Content>
    </Theme>
  );
}
