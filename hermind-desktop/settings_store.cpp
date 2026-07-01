#include "settings_store.h"

#include <QMainWindow>

// ---------------------------------------------------------------------------
// Key paths (group/key)
// ---------------------------------------------------------------------------
static const char kKeyServerUrl[]     = "server/url";
static const char kKeyTheme[]         = "appearance/theme";
static const char kKeyLanguage[]      = "appearance/language";
static const char kKeyAuthToken[]     = "auth/token";
static const char kKeyApiKey[]        = "auth/api_key";
static const char kKeyGeometry[]      = "window/geometry";
static const char kKeyState[]         = "window/state";

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------
static const char kDefaultServerUrl[] = "http://localhost:3001";
static const char kDefaultTheme[]     = "system";
static const char kDefaultLanguage[]  = "en";

// ---------------------------------------------------------------------------
// Singleton
// ---------------------------------------------------------------------------
SettingsStore &SettingsStore::instance()
{
    static SettingsStore store;
    return store;
}

SettingsStore::SettingsStore()
    : QObject(nullptr)
    , m_settings(QStringLiteral("Hermind"), QStringLiteral("HermindDesktop"))
{
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------
QString SettingsStore::serverUrl() const
{
    return m_settings.value(QLatin1StringView(kKeyServerUrl),
                            QLatin1StringView(kDefaultServerUrl)).toString();
}

void SettingsStore::setServerUrl(const QString &url)
{
    m_settings.setValue(QLatin1StringView(kKeyServerUrl), url);
    emit serverUrlChanged(url);
}

// ---------------------------------------------------------------------------
// Appearance
// ---------------------------------------------------------------------------
QString SettingsStore::theme() const
{
    return m_settings.value(QLatin1StringView(kKeyTheme),
                            QLatin1StringView(kDefaultTheme)).toString();
}

void SettingsStore::setTheme(const QString &theme)
{
    m_settings.setValue(QLatin1StringView(kKeyTheme), theme);
    emit themeChanged(theme);
}

QString SettingsStore::language() const
{
    return m_settings.value(QLatin1StringView(kKeyLanguage),
                            QLatin1StringView(kDefaultLanguage)).toString();
}

void SettingsStore::setLanguage(const QString &language)
{
    m_settings.setValue(QLatin1StringView(kKeyLanguage), language);
    emit languageChanged(language);
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------
QString SettingsStore::authToken() const
{
    return m_settings.value(QLatin1StringView(kKeyAuthToken)).toString();
}

void SettingsStore::setAuthToken(const QString &token)
{
    m_settings.setValue(QLatin1StringView(kKeyAuthToken), token);
    emit authTokenChanged(token);
}

QString SettingsStore::apiKey() const
{
    return m_settings.value(QLatin1StringView(kKeyApiKey)).toString();
}

void SettingsStore::setApiKey(const QString &key)
{
    m_settings.setValue(QLatin1StringView(kKeyApiKey), key);
}

// ---------------------------------------------------------------------------
// Window geometry
// ---------------------------------------------------------------------------
QByteArray SettingsStore::windowGeometry() const
{
    return m_settings.value(QLatin1StringView(kKeyGeometry)).toByteArray();
}

void SettingsStore::setWindowGeometry(const QByteArray &geometry)
{
    m_settings.setValue(QLatin1StringView(kKeyGeometry), geometry);
}

QByteArray SettingsStore::windowState() const
{
    return m_settings.value(QLatin1StringView(kKeyState)).toByteArray();
}

void SettingsStore::setWindowState(const QByteArray &state)
{
    m_settings.setValue(QLatin1StringView(kKeyState), state);
}

// ---------------------------------------------------------------------------
// Convenience
// ---------------------------------------------------------------------------
void SettingsStore::saveWindowGeometry(const QMainWindow &window)
{
    setWindowGeometry(window.saveGeometry());
    setWindowState(window.saveState());
}

void SettingsStore::restoreWindowGeometry(QMainWindow &window)
{
    const QByteArray geo = windowGeometry();
    if (!geo.isEmpty())
        window.restoreGeometry(geo);

    const QByteArray state = windowState();
    if (!state.isEmpty())
        window.restoreState(state);
}

// ---------------------------------------------------------------------------
// Management
// ---------------------------------------------------------------------------
void SettingsStore::clear()
{
    m_settings.clear();
}

QSettings &SettingsStore::settings()
{
    return m_settings;
}
