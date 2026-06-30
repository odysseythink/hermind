#ifndef AUTH_MANAGER_H
#define AUTH_MANAGER_H

#include <QObject>
#include <QUrl>
#include <memory>

#include "auth_state.h"
#include "hermind_user.h"

class HermindApiClient;
class SettingsStore;

class AuthManager : public QObject
{
    Q_OBJECT

public:
    static AuthManager &instance();

    AuthState state() const;
    bool isAuthenticated() const;
    HermindUser currentUser() const;
    QString authToken() const;
    QString lastError() const;

    /// Initialize the singleton with explicit dependencies. Call once before use.
    void initialize(HermindApiClient *apiClient, SettingsStore *settings);

    void setAuthToken(const QString &token);
    void setUser(const HermindUser &user);

signals:
    void authStateChanged(AuthState state);
    void userChanged(const HermindUser &user);
    void authTokenChanged(const QString &token);
    void authError(const QString &message);

public slots:
    void login(const QString &username = QString(),
               const QString &password = QString());
    void logout();
    void restoreSession();
    void refreshUser();

protected:
    explicit AuthManager(QObject *parent = nullptr);
    ~AuthManager() override = default;
    Q_DISABLE_COPY(AuthManager)

    void setState(AuthState state);
    void setLastError(const QString &message);

    HermindApiClient *m_apiClient = nullptr;
    SettingsStore *m_settings = nullptr;
    AuthState m_state = AuthState::Unauthenticated;
    HermindUser m_currentUser;
    QString m_authToken;
    QString m_lastError;
};

#endif // AUTH_MANAGER_H
