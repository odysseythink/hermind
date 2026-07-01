#ifndef SETTINGS_STORE_H
#define SETTINGS_STORE_H

#include <QObject>
#include <QSettings>
#include <QString>
#include <QByteArray>

class QMainWindow;

class SettingsStore : public QObject
{
    Q_OBJECT

public:
    static SettingsStore &instance();

    // --- Server ---
    QString serverUrl() const;
    void setServerUrl(const QString &url);

    // --- Appearance ---
    QString theme() const;
    void setTheme(const QString &theme);

    QString language() const;
    void setLanguage(const QString &language);

    // --- Auth ---
    QString authToken() const;
    void setAuthToken(const QString &token);

    QString apiKey() const;
    void setApiKey(const QString &key);

    // --- Window geometry ---
    QByteArray windowGeometry() const;
    void setWindowGeometry(const QByteArray &geometry);

    QByteArray windowState() const;
    void setWindowState(const QByteArray &state);

    // --- Convenience ---
    void saveWindowGeometry(const QMainWindow &window);
    void restoreWindowGeometry(QMainWindow &window);

    // --- Management ---
    /// Reset all stored settings to defaults.
    void clear();

    /// Direct access to the underlying QSettings object.
    QSettings &settings();

signals:
    void themeChanged(const QString &theme);
    void languageChanged(const QString &language);
    void serverUrlChanged(const QString &url);
    void authTokenChanged(const QString &token);

private:
    SettingsStore();
    ~SettingsStore() override = default;
    Q_DISABLE_COPY(SettingsStore)

    QSettings m_settings;
};

#endif // SETTINGS_STORE_H
