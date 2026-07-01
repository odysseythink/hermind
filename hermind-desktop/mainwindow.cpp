#include "mainwindow.h"
#include "ui_mainwindow.h"

#include "main_chat_widget.h"
#include "main_setting_widget.h"
#include "navigation_manager.h"

#include <QDebug>

MainWindow::MainWindow(QWidget *parent)
    : QMainWindow(parent)
    , ui(new Ui::MainWindow)
{
    ui->setupUi(this);

    setWindowTitle(tr("Hermind Desktop"));

    auto *chatWidget = qobject_cast<MainChatWidget*>(ui->stackedWidget->widget(0));
    auto *settingWidget = qobject_cast<MainSettingWidget*>(ui->stackedWidget->widget(1));

    if (chatWidget) {
        registerPage(NavigationPage::DefaultChat, chatWidget);
        connect(chatWidget, &MainChatWidget::bottomSettingClicked,
                this, []() {
            NavigationManager::instance().navigateTo(
                NavigationRoute{NavigationPage::GeneralSettings});
        });
    }

    if (settingWidget) {
        registerPage(NavigationPage::GeneralSettings, settingWidget);
        connect(settingWidget, &MainSettingWidget::bottomReturnClicked,
                this, []() {
            NavigationManager::instance().goBack();
        });
    }

    connect(&NavigationManager::instance(), &NavigationManager::currentRouteChanged,
            this, &MainWindow::onCurrentRouteChanged);

    onCurrentRouteChanged(NavigationManager::instance().currentRoute());
}

void MainWindow::registerPage(NavigationPage page, QWidget *widget)
{
    m_pageRegistry.insert(page, widget);
}

int MainWindow::pageIndex(NavigationPage page) const
{
    QWidget *widget = m_pageRegistry.value(page, nullptr);
    if (!widget)
        return -1;
    return ui->stackedWidget->indexOf(widget);
}

void MainWindow::onCurrentRouteChanged(const NavigationRoute &route)
{
    const int index = pageIndex(route.page);
    if (index < 0) {
        qWarning() << "No widget registered for page" << static_cast<int>(route.page);
        return;
    }

    ui->stackedWidget->setCurrentIndex(index);
    qDebug() << "switched to page" << static_cast<int>(route.page) << "at index" << index;
}

MainWindow::~MainWindow()
{
    delete ui;
}
