import { useTranslation } from 'react-i18next';

export function useDescriptorT() {
  const { t } = useTranslation('descriptors');
  return {
    sectionLabel(sectionKey: string, fallback: string): string {
      return t(`${sectionKey}.label`, { defaultValue: fallback, fallbackLng: [] });
    },
    sectionSummary(sectionKey: string, fallback: string): string {
      return t(`${sectionKey}.summary`, { defaultValue: fallback, fallbackLng: [] });
    },
    fieldLabel(sectionKey: string, fieldName: string, fallback: string): string {
      return t(`${sectionKey}.fields.${fieldName}.label`, { defaultValue: fallback, fallbackLng: [] });
    },
    fieldHelp(sectionKey: string, fieldName: string, fallback: string): string {
      return t(`${sectionKey}.fields.${fieldName}.help`, { defaultValue: fallback, fallbackLng: [] });
    },
    enumValue(
      sectionKey: string,
      fieldName: string,
      value: string,
      fallback: string,
    ): string {
      return t(
        `${sectionKey}.fields.${fieldName}.enum.${value}`,
        { defaultValue: fallback, fallbackLng: [] },
      );
    },
    groupLabel(groupId: string, fallback: string): string {
      return t(`groups.${groupId}`, { defaultValue: fallback, fallbackLng: [] });
    },
  };
}
