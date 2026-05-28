import { useTheme } from "@/hooks/useTheme";

export function OnboardingLogoSVG() {
  const { isLight } = useTheme();
  return (
    <img
      src="/hermind-icon.svg"
      alt="Hermind"
      className="w-full h-auto"
      style={{ opacity: isLight ? 0.5 : 0.28 }}
    />
  );
}
