import { sandboxDiagnosticMessage } from "../../lib/sandbox-diagnostic";

import { useTranslation } from "react-i18next";

export function SandboxDiagnostic({ diagnostic }: { diagnostic?: string }) {
  const { i18n } = useTranslation("sandbox");
  const locale = i18n.resolvedLanguage?.startsWith("zh") ? "zh" : "en";
  const message = sandboxDiagnosticMessage(diagnostic, locale);
  if (!message) return null;
  return <div className="sandbox-diagnostic" role="status">
    <strong>{message.label}</strong><small>{message.advice}</small>
  </div>;
}
