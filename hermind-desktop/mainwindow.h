#ifndef MAINWINDOW_H
#define MAINWINDOW_H

#include <QMainWindow>
#include <QHash>

#include "navigation_route.h"

QT_BEGIN_NAMESPACE
namespace Ui {
class MainWindow;
}
QT_END_NAMESPACE

class MainWindow : public QMainWindow
{
    Q_OBJECT

public:
    explicit MainWindow(QWidget *parent = nullptr);
    ~MainWindow() override;

private slots:
    void onCurrentRouteChanged(const NavigationRoute &route);

private:
    void registerPage(NavigationPage page, QWidget *widget);
    int pageIndex(NavigationPage page) const;

    Ui::MainWindow *ui;
    QHash<NavigationPage, QWidget *> m_pageRegistry;
};

#endif // MAINWINDOW_H
