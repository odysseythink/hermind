#include "mainwindow.h"
#include "ui_mainwindow.h"

#include "main_chat_widget.h"
#include "main_setting_widget.h"

#include <QDebug>

MainWindow::MainWindow(QWidget *parent)
    : QMainWindow(parent)
    , ui(new Ui::MainWindow)
{
    ui->setupUi(this);

    setWindowTitle(tr("AnythingLLM | Superpowers for your OS using local AI"));

    auto *chatWidget = qobject_cast<MainChatWidget*>(ui->stackedWidget->widget(0));
    if (chatWidget) {
        connect(chatWidget, &MainChatWidget::bottomSettingClicked,
                this, &MainWindow::onBottomSettingClicked);
    }

    auto *settingWidget = qobject_cast<MainSettingWidget*>(ui->stackedWidget->widget(1));
    if (settingWidget) {
        connect(settingWidget, &MainSettingWidget::bottomReturnClicked,
                this, &MainWindow::onBottomReturnClicked);
    }
}

void MainWindow::onBottomReturnClicked()
{
    ui->stackedWidget->setCurrentIndex(0);
    qDebug() << "switched to page index 0";
}

void MainWindow::onBottomSettingClicked()
{
    ui->stackedWidget->setCurrentIndex(1);
    qDebug() << "switched to page index 1";
}

MainWindow::~MainWindow()
{
    delete ui;
}

