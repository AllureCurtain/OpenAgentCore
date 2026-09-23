import { createPortal } from "react-dom";
import type { ComponentProps } from "react";

import { Modal } from "../../../components/Modal";
import i18n from "../../../i18n";
import { EnvironmentPanel } from "./EnvironmentPanel";

export interface EnvironmentDialogProps extends ComponentProps<typeof EnvironmentPanel> {
  open: boolean;
  onClose: () => void;
  defaultLauncherGuideOpen?: boolean;
}

export function EnvironmentDialog({
  open,
  onClose,
  environment,
  observation,
  connectionActions,
  dockerGuideProfile,
  defaultLauncherGuideOpen,
  environmentFilesEnabled,
  onListFiles,
  onCreateFile,
}: EnvironmentDialogProps) {
  const t = i18n.getFixedT(null, "sessions");
  const panelProps = {
    environment,
    observation,
    connectionActions,
    dockerGuideProfile,
    defaultLauncherGuideOpen,
    environmentFilesEnabled,
    onListFiles,
    onCreateFile,
  };
  const dialog = (
    <div className="environment-dialog">
      <Modal
        open={open}
        title={t("environment.title")}
        onClose={onClose}
        footer={
          <button className="button primary" type="button" onClick={onClose}>
            {t("common.done")}
          </button>
        }
      >
        <EnvironmentPanel {...panelProps} />
      </Modal>
    </div>
  );

  return typeof document === "undefined" ? dialog : createPortal(dialog, document.body);
}
