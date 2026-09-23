import { Languages } from "lucide-react";
import { useTranslation } from "react-i18next";

import { setLanguage, type SupportedLanguage } from "../i18n";

export function LanguageMenu() {
  const { i18n, t } = useTranslation("navigation");
  const language: SupportedLanguage = i18n.resolvedLanguage?.startsWith("zh") ? "zh-CN" : "en";
  const options: Array<{ value: SupportedLanguage; shortLabel: string; label: string }> = [
    { value: "en", shortLabel: "EN", label: t("english") },
    { value: "zh-CN", shortLabel: "中", label: t("chinese") },
  ];

  return (
    <div className="language-menu" role="group" aria-label={t("language")}>
      <Languages size={14} strokeWidth={1.5} aria-hidden="true" />
      {options.map((option) => (
        <button
          className={language === option.value ? "active" : ""}
          key={option.value}
          type="button"
          aria-pressed={language === option.value}
          aria-label={option.label}
          title={option.label}
          onClick={() => void setLanguage(option.value)}
        >
          {option.shortLabel}
        </button>
      ))}
    </div>
  );
}
