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
    if (!m_apiClient) {
        setLastError(QStringLiteral("AuthManager not initialized"));
        setState(AuthState::Error);
        emit authError(m_lastError);
        return;
    }

    setState(AuthState::Authenticating);
    setLastError(QString());

    m_apiClient->requestToken(username, password,
        [this](const QString &token, const QString &message, const ApiError &error) {
            if (!error.isEmpty() || token.isEmpty()) {
                const QString errMsg = !error.isEmpty() ? error.message() : message;
                setLastError(errMsg.isEmpty() ? QStringLiteral("Login failed") : errMsg);
                setState(AuthState::Error);
                emit authError(m_lastError);
                return;
            }

            if (m_settings)
                m_settings->setAuthToken(token);
            setAuthToken(token);
            m_apiClient->setAuthToken(token);
            setState(AuthState::Authenticated);

            // In multi-user mode the backend also returns a user in the
            // request-token response. If available, set it immediately and
            // then refresh to stay in sync with the server.
            if (!message.isEmpty()) {
                // message is the server message; user comes from refreshUser.
                refreshUser();
            }
        });
}

void AuthManager::logout()
{
    if (m_settings)
        m_settings->setAuthToken(QString());
    if (m_apiClient)
        m_apiClient->setAuthToken(QString());

    setAuthToken(QString());
    setUser(HermindUser());
    setLastError(QString());
    setState(AuthState::Unauthenticated);
}

void AuthManager::restoreSession()
{
    if (!m_settings || !m_apiClient) {
        setState(AuthState::Unauthenticated);
        return;
    }

    const QString token = m_settings->authToken();
    if (token.isEmpty()) {
        setState(AuthState::Unauthenticated);
        return;
    }

    setAuthToken(token);
    m_apiClient->setAuthToken(token);
    setState(AuthState::Authenticated);
    refreshUser();
}

void AuthManager::refreshUser()
{
    if (!m_apiClient) {
        setState(AuthState::Unauthenticated);
        return;
    }

    m_apiClient->refreshUser([this](const HermindUser &user, const QString &message, const ApiError &error) {
        if (!error.isEmpty()) {
            if (!message.isEmpty()) {
                // Backend reported session invalid (multi-user): force logout.
                logout();
                setLastError(message);
                emit authError(message);
            } else {
                // Network / server error during refresh: keep existing session but signal error.
                emit authError(error.message());
            }
            return;
        }

        if (user.id() == 0) {
            // Single-user mode: backend returns user=null, represented as id==0.
            // Keep current user empty and do not signal a change unless it was non-empty.
            if (m_currentUser.id() != 0)
                setUser(HermindUser());
            return;
        }

        setUser(user);
    });
}
