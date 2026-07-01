#include "mainwindow.h"
#include "settings_store.h"
#include "theme_manager.h"

#include <QApplication>

int main(int argc, char *argv[])
{
    QApplication a(argc, argv);

    // Initialize theme manager before showing any windows
    ThemeManager::instance().initialize(&SettingsStore::instance(), &a);

    MainWindow w;
    w.show();
    return QApplication::exec();
}
