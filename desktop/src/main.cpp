#include <QApplication>
#include "appwindow.h"
#include "hermindprocess.h"
#include "httplib.h"

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    app.setApplicationName("hermind");
    app.setOrganizationName("hermind");

    AppWindow window;
    HermindProcess backend;
    HermindClient *client = nullptr;

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
