import { Layers3, Server } from "lucide-react";
import { useTranslation } from "react-i18next";
export type SystemView = "system" | "sandbox";
export function SystemNavigation({ active, onSelect }: { active: SystemView | null; onSelect: (view: SystemView) => void }) {
  const { t } = useTranslation("navigation");
  return <nav className="main-nav" aria-label={t("systemNavigation")}>
    <p className="nav-label">{t("system")}</p>
    {([{ id: "system", label: "system", icon: Layers3 }, { id: "sandbox", label: "sandbox", icon: Server }] as const).map(({ id, label, icon: Icon }) => (
      <button key={id} type="button" className={active === id ? "active" : ""} onClick={() => onSelect(id)} aria-label={t(label)} aria-current={active === id ? "page" : undefined}>
        <Icon size={15} strokeWidth={1.5} aria-hidden="true" /><span>{t(label)}</span>
      </button>
    ))}
  </nav>;
}
