import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { consolePageDefinition } from "console/routes";
import { useConsolePostureSummary } from "features/console-summary/public";
import { useI18n } from "platform/i18n";
import { renderConsolePage } from "./pageRegistry";
import { useConsoleRouter } from "./router/useConsoleRouter";
import { ConsoleHeader } from "./shell/ConsoleHeader";
import { ConsoleShell } from "./shell/ConsoleShell";

export function App() {
  const queryClient = useQueryClient();
  const { locale, setLocale, t } = useI18n();
  const { destination, navigate } = useConsoleRouter();
  const { summary } = useConsolePostureSummary();
  const page = consolePageDefinition(destination);

  useEffect(() => {
    document.title = `KiwiGuard - ${t(page.titleKey)}`;
  }, [page.titleKey, t]);

  return (
    <>
      <ConsoleHeader
        healthIsError={summary.healthIsError}
        healthIsLoading={summary.healthIsLoading}
        healthState={summary.healthState}
        locale={locale}
        onLocaleChange={setLocale}
        onRefresh={() => {
          void queryClient.invalidateQueries();
        }}
        t={t}
      />
      <ConsoleShell
        activeDestination={destination}
        onNavigate={navigate}
        summary={summary}
        t={t}
      >
        {renderConsolePage({ destination, navigate, summary, t })}
      </ConsoleShell>
    </>
  );
}
