#include <QGuiApplication>
#include <QQmlApplicationEngine>
#include <QFont>
#include <QQmlContext>
#include <QQuickStyle>
#include <QWindow>
#include <QKeySequence>
#include "HermindProcess.h"
#include "HermindClient.h"
#include "AppState.h"
#include "TrayIcon.h"
#include "ShortcutManager.h"

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
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
    QGuiApplication::setFont(appFont);

    QQuickStyle::setStyle("Basic");

    HermindProcess backend;
    HermindClient *client = nullptr;
    AppState *appState = nullptr;

    QQmlApplicationEngine engine;

    QObject::connect(&backend, &HermindProcess::backendReady,
                     &app, [&engine, &client, &appState](const QHostAddress&, int port) {
        client = new HermindClient(QStringLiteral("http://127.0.0.1:%1").arg(port), &engine);
        appState = new AppState(client, &engine);
        engine.rootContext()->setContextProperty("appState", appState);
        engine.rootContext()->setContextProperty("hermindClient", client);
        appState->boot();
    });

    QObject::connect(&backend, &HermindProcess::backendError,
                     &app, [](const QString &msg) {
        qWarning() << "Backend error:" << msg;
    });

    TrayIcon tray;
    tray.show();
    QObject::connect(&tray, &TrayIcon::showWindowRequested, &app, [&engine]() {
        for (QObject *obj : engine.rootObjects()) {
            if (QWindow *w = qobject_cast<QWindow*>(obj)) {
                w->show();
                w->raise();
                w->requestActivate();
            }
        }
    });
    QObject::connect(&tray, &TrayIcon::quitRequested, &app, &QGuiApplication::quit);

    ShortcutManager shortcuts;
    shortcuts.registerToggle(QKeySequence("Ctrl+Shift+H"));
    QObject::connect(&shortcuts, &ShortcutManager::toggleRequested, &app, [&engine]() {
        for (QObject *obj : engine.rootObjects()) {
            if (QWindow *w = qobject_cast<QWindow*>(obj)) {
                if (w->isVisible()) w->hide();
                else { w->show(); w->raise(); w->requestActivate(); }
            }
        }
    });

    engine.load(QUrl(QStringLiteral("qrc:/Hermind/qml/main.qml")));
    if (engine.rootObjects().isEmpty()) return -1;

    backend.start();

    int ret = app.exec();
    backend.shutdown();
    return ret;
}
