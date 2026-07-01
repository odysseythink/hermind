#ifndef THEME_MANAGER_H
#define THEME_MANAGER_H

#include <QObject>
#include <QString>

class QApplication;
class SettingsStore;

class ThemeManager : public QObject
{
    Q_OBJECT

public:
    static ThemeManager &instance();

    /// Initialize: read stored theme/language from SettingsStore and apply theme.
    /// Must be called once after QApplication is created.
    void initialize(SettingsStore *settings, QApplication *app = nullptr);

    // --- Theme ---
    /// Returns the stored theme key: "system", "light", or "dark".
    QString theme() const;

    /// Sets and persists the theme. Applies palette to QApplication.
    void setTheme(const QString &theme);

    /// Returns the *effective* theme after resolving "system" to the OS preference.
    /// Guaranteed to be "light" or "dark".
    QString effectiveTheme() const;

    /// Convenience: true if the effective theme is dark.
    bool isDarkMode() const;

    // --- Language ---
    /// Returns the stored language code (e.g. "en", "zh").
    QString language() const;

    /// Sets and persists the language code.
    void setLanguage(const QString &language);

    // --- Helpers ---
    /// Best-effort detection of the OS colour scheme. Never fails — falls back to "light".
    static QString detectSystemTheme();

signals:
    void themeChanged(const QString &theme);
    void languageChanged(const QString &language);

private:
    ThemeManager();
    ~ThemeManager() override = default;
    Q_DISABLE_COPY(ThemeManager)

    void applyThemeToApplication(const QString &effectiveTheme);

    SettingsStore *m_settings = nullptr;
    QApplication *m_app = nullptr;
    QString m_theme;
    QString m_language;
};

#endif // THEME_MANAGER_H
