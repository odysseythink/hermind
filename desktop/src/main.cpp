#include <QApplication>
#include <QQmlApplicationEngine>
#include <QFont>
#include <QQmlContext>
#include <QQuickStyle>
#include <QWindow>
#include <QKeySequence>
#include <QTranslator>
#include <QtQml/qqml.h>
#include <QJsonDocument>
#include "HermindCGOClient.h"
extern "C" {
#include "libgo-desktop-interface.h"
}
#include "AppState.h"
#include "TrayIcon.h"
#include "ShortcutManager.h"
#include "Theme.h"

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

    QQuickStyle::setStyle("Basic");

    QTranslator translator;

    QQmlApplicationEngine engine;

    Theme theme;
    AppState appState(nullptr, &app);

    QJSValue global = engine.globalObject();
    global.setProperty("Theme", engine.newQObject(&theme));
    global.setProperty("appState", engine.newQObject(&appState));

    // Initialize Go backend via CGO
    char* initStatus = HermindInit(const_cast<char*>(""));
    QJsonDocument initDoc = QJsonDocument::fromJson(QByteArray(initStatus));
    HermindFree(initStatus);

    if (!initDoc.isNull() && initDoc.object().value(QStringLiteral("status")).toString() == QStringLiteral("ok")) {
        qDebug() << "Go backend initialized via CGO";
    } else {
        qWarning() << "Go backend init failed:" << initDoc.toJson();
    }

    // Create CGO client directly (no process startup needed)
    HermindCGOClient *client = new HermindCGOClient(&engine);
    appState.setClient(client);
    engine.rootContext()->setContextProperty("hermindClient", client);

    QObject::connect(&appState, &AppState::languageChanged, &app, [&app, &translator](const QString &lang) {
        app.removeTranslator(&translator);
        if (translator.load(QStringLiteral(":/i18n/hermind_%1").arg(lang))) {
            app.installTranslator(&translator);
        }
    });

    appState.boot();

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

    int ret = app.exec();
    return ret;
}
