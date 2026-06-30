#include "auth_manager.h"

#include "hermind_api_client.h"
#include "settings_store.h"

AuthManager &AuthManager::instance()
{
    static AuthManager mgr;
    return mgr;
}

AuthManager::AuthManager(QObject *parent)
    : QObject(parent)
{
}

void AuthManager::initialize(HermindApiClient *apiClient, SettingsStore *settings)
{
    m_apiClient = apiClient;
    m_settings = settings;
}

AuthState AuthManager::state() const { return m_state; }
bool AuthManager::isAuthenticated() const { return m_state == AuthState::Authenticated; }
HermindUser AuthManager::currentUser() const { return m_currentUser; }
QString AuthManager::authToken() const { return m_authToken; }
QString AuthManager::lastError() const { return m_lastError; }

void AuthManager::setState(AuthState state)
{
    if (m_state == state)
        return;
    m_state = state;
    emit authStateChanged(m_state);
}

void AuthManager::setUser(const HermindUser &user)
{
    if (m_currentUser.id() == user.id() &&
        m_currentUser.username() == user.username() &&
        m_currentUser.role() == user.role()) {
        return;
    }
    m_currentUser = user;
    emit userChanged(m_currentUser);
}

void AuthManager::setAuthToken(const QString &token)
{
    if (m_authToken == token)
        return;
    m_authToken = token;
    emit authTokenChanged(m_authToken);
}

void AuthManager::setLastError(const QString &message)
{
    m_lastError = message;
}

void AuthManager::login(const QString &username, const QString &password)
{
    Q_UNUSED(username)
    Q_UNUSED(password)
    setState(AuthState::Authenticating);
    setLastError(QString());
}

void AuthManager::logout()
{
    setAuthToken(QString());
    setUser(HermindUser());
    setLastError(QString());
    setState(AuthState::Unauthenticated);
}

void AuthManager::restoreSession()
{
    if (!m_settings) {
        setState(AuthState::Unauthenticated);
        return;
    }
    const QString token = m_settings->authToken();
    if (token.isEmpty()) {
        setState(AuthState::Unauthenticated);
        return;
    }
    setAuthToken(token);
    setState(AuthState::Authenticated);
}

void AuthManager::refreshUser()
{
    // Placeholder: filled in Task 4.
}
