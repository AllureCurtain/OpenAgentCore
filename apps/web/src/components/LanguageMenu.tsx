import { useTranslation } from "react-i18next";

import { setLanguage, type SupportedLanguage } from "../i18n";

export function LanguageMenu() {
  const { i18n, t } = useTranslation("navigation");
  const language: SupportedLanguage = i18n.resolvedLanguage?.startsWith("zh") ? "zh-CN" : "en";
  const nextLanguage: SupportedLanguage = language === "en" ? "zh-CN" : "en";
  const label = language === "en" ? t("switchToChinese") : t("switchToEnglish");

  return (
    <button
      className="language-menu"
      type="button"
      aria-label={label}
      title={label}
      onClick={() => void setLanguage(nextLanguage)}
    >
      <span className={language === "en" ? "active" : undefined}>EN</span>
      <span className="language-menu-divider" aria-hidden="true">/</span>
      <span className={language === "zh-CN" ? "active" : undefined}>中</span>
    </button>
  );
}
