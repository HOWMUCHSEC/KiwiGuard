import { Renew } from "@carbon/icons-react";
import { Header, HeaderGlobalAction, HeaderGlobalBar, HeaderName, Select, SelectItem, Tag } from "@carbon/react";

import type { Locale } from "platform/i18n";
import { healthLabelKey, healthTagType } from "shared/ui/status";

type ConsoleHeaderProps = {
  healthIsError: boolean;
  healthIsLoading: boolean;
  healthState: string;
  locale: Locale;
  onLocaleChange: (locale: Locale) => void;
  onRefresh: () => void;
  t: (key: string) => string;
};

export function ConsoleHeader({ healthIsError, healthIsLoading, healthState, locale, onLocaleChange, onRefresh, t }: ConsoleHeaderProps) {
  return (
    <Header aria-label={t("app.aria")}>
      <HeaderName href="#" prefix="KiwiGuard">
        {t("app.title")}
      </HeaderName>
      <HeaderGlobalBar>
        <div className="kg-language-switcher">
          <Select
            id="language-switcher"
            hideLabel
            labelText={t("language.label")}
            value={locale}
            onChange={(event) => onLocaleChange(event.target.value === "zh-CN" ? "zh-CN" : "en")}
          >
            <SelectItem value="en" text={t("language.english")} />
            <SelectItem value="zh-CN" text={t("language.chinese")} />
          </Select>
        </div>
        <div className="kg-header-status">
          <Tag type={healthIsError ? "red" : healthTagType(healthState)}>
            {healthIsLoading ? t("health.checking") : healthIsError ? t("health.unavailable") : t(healthLabelKey(healthState))}
          </Tag>
        </div>
        <HeaderGlobalAction aria-label={t("action.refresh")} onClick={onRefresh}>
          <Renew />
        </HeaderGlobalAction>
      </HeaderGlobalBar>
    </Header>
  );
}
