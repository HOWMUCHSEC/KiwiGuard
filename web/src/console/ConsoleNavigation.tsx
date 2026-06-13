import { Button } from "@carbon/react";

import { useI18n } from "platform/i18n";
import type { ConsoleDestination } from "./routes";
import { consoleDomains, domainTabs } from "./routes";

type ConsoleNavigationProps = {
  active: ConsoleDestination;
  onNavigate: (destination: ConsoleDestination) => void;
};

export function ConsoleNavigation({ active, onNavigate }: ConsoleNavigationProps) {
  const { t } = useI18n();
  const activeTabs = domainTabs(active.domain);

  return (
    <nav className="kg-domain-nav" aria-label={t("app.title")}>
      <div className="kg-domain-nav-primary">
        {consoleDomains.map((domainEntry) => {
          const tab = domainEntry.tabs[0].tab;
          const isSelected = active.domain === domainEntry.domain;

          return (
            <Button
              key={domainEntry.domain}
              className="kg-domain-nav-item"
              kind={isSelected ? "primary" : "ghost"}
              size="sm"
              aria-current={isSelected ? "page" : undefined}
              onClick={() => onNavigate({ domain: domainEntry.domain, tab })}
            >
              {t(domainEntry.labelKey)}
            </Button>
          );
        })}
      </div>

      {activeTabs.length > 1 ? (
        <div className="kg-domain-nav-tabs" aria-label={t(`nav.${active.domain}`)}>
          {activeTabs.map((tabEntry) => {
            const isSelected = active.tab === tabEntry.tab;

            return (
              <Button
                key={tabEntry.tab}
                className="kg-domain-nav-tab"
                kind={isSelected ? "secondary" : "ghost"}
                size="sm"
                aria-current={isSelected ? "page" : undefined}
                onClick={() => onNavigate({ domain: active.domain, tab: tabEntry.tab })}
              >
                {t(tabEntry.labelKey)}
              </Button>
            );
          })}
        </div>
      ) : null}
    </nav>
  );
}
