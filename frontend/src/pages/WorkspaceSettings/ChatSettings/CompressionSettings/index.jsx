import { useState } from "react";
import { useTranslation } from "react-i18next";

const ENABLED_OPTIONS = [
  { value: "default", labelKey: "chat.compression.followGlobal" },
  { value: "true", labelKey: "chat.compression.enabled" },
  { value: "false", labelKey: "chat.compression.disabled" },
];

export default function CompressionSettings({
  workspace,
  settings,
  setHasChanges,
}) {
  const { t } = useTranslation();

  // Global default from system settings (string "true" or "false")
  const globalEnabled = settings?.context_compress_enabled === "true";

  // Local state: "default" | "true" | "false"
  const [compressEnabled, setCompressEnabled] = useState(() => {
    if (workspace?.compressEnabled === true) return "true";
    if (workspace?.compressEnabled === false) return "false";
    return "default";
  });

  const [compressThreshold, setCompressThreshold] = useState(
    workspace?.compressThreshold?.toString() ?? ""
  );
  const [compressContextLen, setCompressContextLen] = useState(
    workspace?.compressContextLen?.toString() ?? ""
  );

  const isOverriding = compressEnabled !== "default";

  return (
    <div className="flex flex-col gap-y-4">
      <div className="flex flex-col">
        <label className="block input-label">{t("chat.compression.title")}</label>
        <p className="text-white text-opacity-60 text-xs font-medium py-1.5">
          {t("chat.compression.description")}
        </p>
      </div>

      {/* Three-state toggle */}
      <div className="w-fit flex gap-x-1 items-center p-1 rounded-lg bg-theme-settings-input-bg">
        <input type="hidden" name="compressEnabled" value={compressEnabled} />
        {ENABLED_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            type="button"
            disabled={compressEnabled === opt.value}
            onClick={() => {
              setCompressEnabled(opt.value);
              setHasChanges(true);
            }}
            className="border-none transition-bg duration-200 px-4 py-1 text-sm text-white/60 disabled:text-white bg-transparent disabled:bg-[#687280] rounded-md hover:bg-white/10 light:hover:bg-black/10"
          >
            {t(opt.labelKey)}
          </button>
        ))}
      </div>

      {/* Global status hint */}
      {compressEnabled === "default" && (
        <p className="text-xs text-white/60">
          {t("chat.compression.globalStatus")}: {" "}
          <b>
            {globalEnabled
              ? t("chat.compression.enabled")
              : t("chat.compression.disabled")}
          </b>
        </p>
      )}

      {/* Threshold override */}
      <div className="flex flex-col gap-y-1">
        <label className="block text-xs font-medium text-white/80">
          {t("chat.compression.threshold")}
        </label>
        <p className="text-white text-opacity-60 text-xs">
          {t("chat.compression.thresholdDesc")}
        </p>
        <input
          type="hidden"
          name="compressThreshold"
          value={compressThreshold}
        />
        <input
          type="number"
          min={0.3}
          max={0.95}
          step={0.05}
          value={compressThreshold}
          onChange={(e) => {
            setCompressThreshold(e.target.value);
            setHasChanges(true);
          }}
          onWheel={(e) => e.target.blur()}
          placeholder={t("chat.compression.thresholdPlaceholder")}
          className="border-none bg-theme-settings-input-bg text-white placeholder:text-theme-settings-input-placeholder text-sm rounded-lg focus:outline-primary-button active:outline-primary-button outline-none block w-full p-2.5"
        />
      </div>

      {/* Context length override */}
      <div className="flex flex-col gap-y-1">
        <label className="block text-xs font-medium text-white/80">
          {t("chat.compression.contextLength")}
        </label>
        <p className="text-white text-opacity-60 text-xs">
          {t("chat.compression.contextLengthDesc")}
        </p>
        <input
          type="hidden"
          name="compressContextLen"
          value={compressContextLen}
        />
        <input
          type="number"
          min={1}
          step={1}
          value={compressContextLen}
          onChange={(e) => {
            setCompressContextLen(e.target.value);
            setHasChanges(true);
          }}
          onWheel={(e) => e.target.blur()}
          placeholder={t("chat.compression.contextLengthPlaceholder")}
          className="border-none bg-theme-settings-input-bg text-white placeholder:text-theme-settings-input-placeholder text-sm rounded-lg focus:outline-primary-button active:outline-primary-button outline-none block w-full p-2.5"
        />
      </div>
    </div>
  );
}
