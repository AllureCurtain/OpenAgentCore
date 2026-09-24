import { Bot, LayoutDashboard, Layers3, MessageSquare, Vault, type LucideIcon } from "lucide-react";
import { useTranslation } from "react-i18next";

export type ProductView = "dashboard" | "agents" | "sessions" | "vaults" | "templates";

const productViews: Array<{ id: ProductView; label: "dashboard" | "agents" | "sessions" | "templates" | "vaults"; icon: LucideIcon }> = [
  { id: "dashboard", label: "dashboard", icon: LayoutDashboard },
  { id: "agents", label: "agents", icon: Bot },
  { id: "sessions", label: "sessions", icon: MessageSquare },
  { id: "templates", label: "templates", icon: Layers3 },
  { id: "vaults", label: "vaults", icon: Vault },
];

export function ProductNavigation({
  active,
  onSelect,
  showVaults = false,
  showTemplates = false,
}: {
  active: ProductView | null;
  onSelect: (view: ProductView) => void;
  showVaults?: boolean;
  showTemplates?: boolean;
}) {
  const { t } = useTranslation("navigation");
  return (
    <nav className="main-nav product-navigation" aria-label={t("productLabel")}>
      <p className="nav-label">{t("workspace")}</p>
      {productViews.filter((item) => (item.id !== "vaults" || showVaults) && (item.id !== "templates" || showTemplates)).map((item) => {
        const Icon = item.icon;
        return (
          <button
            type="button"
            className={active === item.id ? "active" : ""}
            key={item.id}
            onClick={() => onSelect(item.id)}
            aria-label={t(item.label)}
            aria-current={active === item.id ? "page" : undefined}
          >
            <Icon size={15} strokeWidth={1.5} aria-hidden="true" />
            <span>{t(item.label)}</span>
          </button>
        );
      })}
    </nav>
  );
}
