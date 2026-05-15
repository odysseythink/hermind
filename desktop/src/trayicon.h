#ifndef TRAYICON_H
#define TRAYICON_H

#include <QObject>

class QSystemTrayIcon;
class QMenu;
class QAction;

class TrayIcon : public QObject
{
    Q_OBJECT
public:
    explicit TrayIcon(QObject *parent = nullptr);
    void show();
    void notify(const QString &title, const QString &message);

signals:
    void showWindowRequested();
    void quitRequested();

private:
    QSystemTrayIcon *m_tray;
    QMenu *m_menu;
};

#endif
