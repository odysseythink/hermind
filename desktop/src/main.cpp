#include <QApplication>
#include <QFile>
#include <QFont>
#include "appwindow.h"
#include "hermindprocess.h"
#include "httplib.h"
#include "shortcutmanager.h"
#include "trayicon.h"

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    app.setApplicationName("hermind");
    app.setOrganizationName("hermind");

    QFont appFont;
#ifdef Q_OS_MAC
    appFont = QFont("-apple-system");
#elif defined(Q_OS_WIN)
    appFont = QFont("Segoe UI");
#else
    appFont = QFont("system-ui");
#endif
    appFont.setPointSize(10);
    QApplication::setFont(appFont);

    QFile styleFile(":/styles.qss");
    if (styleFile.open(QFile::ReadOnly)) {
        app.setStyleSheet(QString::fromUtf8(styleFile.readAll()));
    }

    AppWindow window;
    HermindProcess backend;
    HermindClient *client = nullptr;

    ShortcutManager shortcuts(&window);
    shortcuts.registerToggle(QKeySequence("Ctrl+Shift+H"));
    QObject::connect(&shortcuts, &ShortcutManager::toggleRequested, &window, [&window]() {
        if (window.isVisible()) {
            window.hide();
        } else {
            window.show();
            window.raise();
            window.activateWindow();
        }
    });

    TrayIcon tray;
    tray.show();
    QObject::connect(&tray, &TrayIcon::showWindowRequested, &window, [&window]() {
        window.show();
        window.raise();
        window.activateWindow();
    });
    QObject::connect(&tray, &TrayIcon::quitRequested, &app, &QApplication::quit);

    QObject::connect(&backend, &HermindProcess::backendReady,
                     &window, [&window, &client](const QHostAddress&, int port) {
        client = new HermindClient(QString("http://127.0.0.1:%1").arg(port), &window);
        window.setClient(client);
    });

    QObject::connect(&backend, &HermindProcess::backendError,
                     &window, [](const QString &msg) {
        qWarning() << "Backend error:" << msg;
    });

    backend.start();
    window.show();

    int ret = app.exec();
    backend.shutdown();
    return ret;
}
