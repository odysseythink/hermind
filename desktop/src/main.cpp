#include <QApplication>
#include <QMainWindow>

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    app.setApplicationName("hermind");
    app.setOrganizationName("hermind");

    QMainWindow window;
    window.setWindowTitle("hermind");
    window.resize(1200, 800);
    window.show();

    return app.exec();
}
