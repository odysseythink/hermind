#include "mainwindow.h"
#include "auth_manager.h"
#include "hermind_api_client.h"
#include "settings_store.h"
#include "theme_manager.h"

#include <QApplication>
#include <QUrl>

int main(int argc, char *argv[])
{
    QApplication a(argc, argv);

    // Initialize theme manager before showing any windows
    ThemeManager::instance().initialize(&SettingsStore::instance(), &a);

    // Wire up the API client and auth singleton, then restore any saved session.
    auto *apiClient = new HermindApiClient(&a);
    apiClient->setBaseUrl(QUrl(SettingsStore::instance().serverUrl() + QStringLiteral("/api")));
    AuthManager::instance().initialize(apiClient, &SettingsStore::instance());
    AuthManager::instance().restoreSession();

    MainWindow w;
    w.show();
    return QApplication::exec();
}
