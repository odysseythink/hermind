#include "trayicon.h"

#include <QSystemTrayIcon>
#include <QMenu>
#include <QAction>
#include <QApplication>

TrayIcon::TrayIcon(QObject *parent)
    : QObject(parent),
      m_tray(new QSystemTrayIcon(this)),
      m_menu(new QMenu())
{
    m_tray->setIcon(QApplication::windowIcon());
    m_tray->setToolTip("hermind");

    QAction *showAction = m_menu->addAction("Show");
    connect(showAction, &QAction::triggered, this, &TrayIcon::showWindowRequested);

    m_menu->addSeparator();

    QAction *quitAction = m_menu->addAction("Quit");
    connect(quitAction, &QAction::triggered, this, &TrayIcon::quitRequested);

    m_tray->setContextMenu(m_menu);

    connect(m_tray, &QSystemTrayIcon::activated,
            this, [this](QSystemTrayIcon::ActivationReason reason) {
        if (reason == QSystemTrayIcon::Trigger || reason == QSystemTrayIcon::DoubleClick) {
            emit showWindowRequested();
        }
    });
}

void TrayIcon::show()
{
    m_tray->show();
}

void TrayIcon::notify(const QString &title, const QString &message)
{
    m_tray->showMessage(title, message);
}
