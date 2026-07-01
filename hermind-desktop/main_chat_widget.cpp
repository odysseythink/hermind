#include "main_chat_widget.h"
#include "ui_main_chat_widget.h"
#include "widgets/icon_button.h"
#include "widgets/search_input.h"
#include "widgets/theme_colors.h"
#include "theme_manager.h"

#include <QDebug>
#include <QPixmap>
#include <QLineEdit>
#include <QBoxLayout>

MainChatWidget::MainChatWidget(QWidget *parent)
    : QWidget(parent)
    , ui(new Ui::MainChatWidget)
{
    ui->setupUi(this);

    replaceToolButtons();
    replaceSearchEdit();

    setupLogo();
    setupThreadList();
    setupStyleSheet();

    connect(ui->popoutButton, &QToolButton::clicked, this, &MainChatWidget::on_popoutButton_clicked);
    connect(ui->newSearchButton, &QToolButton::clicked, this, &MainChatWidget::on_newSearchButton_clicked);
    connect(ui->uploadButton, &QToolButton::clicked, this, &MainChatWidget::on_uploadButton_clicked);
    connect(ui->workspaceSettingsButton, &QToolButton::clicked, this, &MainChatWidget::on_workspaceSettingsButton_clicked);
    connect(ui->newThreadButton, &QPushButton::clicked, this, &MainChatWidget::on_newThreadButton_clicked);
    connect(ui->assistantChatsButton, &QPushButton::clicked, this, &MainChatWidget::on_assistantChatsButton_clicked);
    connect(ui->bottomSettingButton, &QToolButton::clicked, this, &MainChatWidget::on_bottomSettingButton_clicked);
    connect(ui->headerSettingsButton, &QToolButton::clicked, this, &MainChatWidget::on_headerSettingsButton_clicked);
    connect(ui->toolsButton, &QPushButton::clicked, this, &MainChatWidget::on_toolsButton_clicked);
    connect(ui->micButton, &QToolButton::clicked, this, &MainChatWidget::on_micButton_clicked);
    connect(ui->sendButton, &QToolButton::clicked, this, &MainChatWidget::on_sendButton_clicked);
    connect(ui->createAgentButton, &QPushButton::clicked, this, &MainChatWidget::on_createAgentButton_clicked);
    connect(ui->editWorkspaceButton, &QPushButton::clicked, this, &MainChatWidget::on_editWorkspaceButton_clicked);
    connect(ui->uploadFileButton, &QPushButton::clicked, this, &MainChatWidget::on_uploadFileButton_clicked);
}

MainChatWidget::~MainChatWidget()
{
    delete ui;
}

void MainChatWidget::replaceToolButtons()
{
    auto replaceOne = [this](const QString &name, const QString &iconText) {
        QToolButton *oldBtn = findChild<QToolButton *>(name);
        if (!oldBtn)
            return;

        IconButton *newBtn = new IconButton(this);
        newBtn->setObjectName(name);
        newBtn->setIconText(iconText);

        QLayout *layout = oldBtn->parentWidget()->layout();
        if (layout) {
            int idx = -1;
            for (int i = 0; i < layout->count(); ++i) {
                if (layout->itemAt(i)->widget() == oldBtn) {
                    idx = i;
                    break;
                }
            }
            if (idx >= 0) {
                layout->removeWidget(oldBtn);
                if (auto *box = qobject_cast<QBoxLayout *>(layout))
                    box->insertWidget(idx, newBtn);
            }
        }

        oldBtn->deleteLater();

        // Update ui-> pointer (all QToolButton* members)
        if (name == QLatin1String("popoutButton")) ui->popoutButton = newBtn;
        else if (name == QLatin1String("newSearchButton")) ui->newSearchButton = newBtn;
        else if (name == QLatin1String("uploadButton")) ui->uploadButton = newBtn;
        else if (name == QLatin1String("workspaceSettingsButton")) ui->workspaceSettingsButton = newBtn;
        else if (name == QLatin1String("headerSettingsButton")) ui->headerSettingsButton = newBtn;
        else if (name == QLatin1String("micButton")) ui->micButton = newBtn;
        else if (name == QLatin1String("sendButton")) ui->sendButton = newBtn;
        else if (name == QLatin1String("bottomSettingButton")) ui->bottomSettingButton = newBtn;
    };

    replaceOne(QStringLiteral("popoutButton"), QString::fromUtf8("⧉"));
    replaceOne(QStringLiteral("newSearchButton"), QStringLiteral("+"));
    replaceOne(QStringLiteral("uploadButton"), QString::fromUtf8("⬆"));
    replaceOne(QStringLiteral("workspaceSettingsButton"), QString::fromUtf8("⚙"));
    replaceOne(QStringLiteral("headerSettingsButton"), QString::fromUtf8("⚙"));
    replaceOne(QStringLiteral("micButton"), QString::fromUtf8("🎤"));
    replaceOne(QStringLiteral("sendButton"), QString::fromUtf8("➤"));
    replaceOne(QStringLiteral("bottomSettingButton"), QString::fromUtf8("🔧"));
}

void MainChatWidget::replaceSearchEdit()
{
    QLineEdit *oldEdit = ui->searchEdit;
    if (!oldEdit)
        return;

    SearchInput *newEdit = new SearchInput(this);
    newEdit->setObjectName(QStringLiteral("searchEdit"));
    newEdit->setPlaceholderText(oldEdit->placeholderText());

    QLayout *layout = oldEdit->parentWidget()->layout();
    if (layout) {
        int idx = -1;
        for (int i = 0; i < layout->count(); ++i) {
            if (layout->itemAt(i)->widget() == oldEdit) {
                idx = i;
                break;
            }
        }
        if (idx >= 0) {
            layout->removeWidget(oldEdit);
            if (auto *box = qobject_cast<QBoxLayout *>(layout))
                box->insertWidget(idx, newEdit);
        }
    }

    oldEdit->deleteLater();
    ui->searchEdit = newEdit;
}

void MainChatWidget::setupLogo()
{
    QPixmap logo(":/images/logo.svg");
    if (!logo.isNull()) {
        ui->logoLabel->setPixmap(logo.scaled(24, 24, Qt::KeepAspectRatio, Qt::SmoothTransformation));
    }
}

void MainChatWidget::setupThreadList()
{
    ui->threadList->addItem("hello");
    ui->threadList->addItem("Thread");
    ui->threadList->addItem("hello");
    ui->threadList->addItem(QString::fromUtf8("联网搜索惠阳今天的..."));
    ui->threadList->addItem("*New Thread");
    ui->threadList->setCurrentRow(4);
}

void MainChatWidget::setupStyleSheet()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString windowBg = ThemeColors::windowBackground(dark).name();
    const QString sidebarBg = ThemeColors::sidebarBackground(dark).name();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString textSecondary = ThemeColors::textSecondary(dark).name();
    const QString hoverBg = ThemeColors::hoverBackground(dark).name();
    const QString selectedBg = ThemeColors::selectedBackground(dark).name();

    setStyleSheet(QStringLiteral(R"(
        MainChatWidget {
            background-color: %1;
        }

        #sidebarFrame {
            background-color: %2;
            border: none;
        }

        #chatFrame {
            background-color: %1;
            border: none;
        }

        #brandLabel {
            color: %3;
            font-size: 16px;
            font-weight: 600;
        }

        #workspaceLabel {
            color: %3;
            font-size: 13px;
            font-weight: 500;
        }

        #workspaceIcon {
            color: %4;
            font-size: 12px;
        }

        #threadList {
            background-color: transparent;
            border: none;
            outline: none;
        }

        #threadList::item {
            color: %3;
            font-size: 13px;
            padding: 8px 12px;
            border-radius: 6px;
        }

        #threadList::item:selected {
            background-color: %6;
            color: %3;
        }

        #threadList::item:hover {
            background-color: %5;
        }

        #newThreadButton, #assistantChatsButton {
            background-color: %1;
            border: 1px solid %5;
            border-radius: 8px;
            padding: 8px 12px;
            color: %3;
            font-size: 13px;
            text-align: left;
        }

        #newThreadButton:hover, #assistantChatsButton:hover {
            background-color: %5;
        }

        #workspaceNameLabel {
            color: %4;
            font-size: 14px;
            font-weight: 500;
        }

        #versionLabel {
            color: #D4A017;
            font-size: 12px;
            font-weight: 500;
        }

        #welcomeLabel {
            color: %3;
            font-size: 28px;
            font-weight: 500;
        }

        #inputFrame {
            background-color: %1;
            border: 1px solid %5;
            border-radius: 16px;
        }

        #messageEdit {
            background-color: transparent;
            border: none;
            color: %3;
            font-size: 14px;
        }

        #messageEdit::placeholder {
            color: %4;
        }

        #toolsButton {
            background-color: transparent;
            border: none;
            color: %4;
            font-size: 13px;
            padding: 4px 8px;
        }

        #toolsButton:hover {
            background-color: %5;
            border-radius: 6px;
        }

        #createAgentButton, #editWorkspaceButton, #uploadFileButton {
            background-color: %5;
            border: none;
            border-radius: 16px;
            padding: 8px 16px;
            color: %3;
            font-size: 13px;
        }

        #createAgentButton:hover, #editWorkspaceButton:hover, #uploadFileButton:hover {
            background-color: %5;
        }
    )").arg(windowBg, sidebarBg, textPrimary, textSecondary, hoverBg, selectedBg));
}

void MainChatWidget::on_popoutButton_clicked() { qDebug() << "popout clicked"; }
void MainChatWidget::on_newSearchButton_clicked() { qDebug() << "new search clicked"; }
void MainChatWidget::on_uploadButton_clicked() { qDebug() << "upload clicked"; }
void MainChatWidget::on_workspaceSettingsButton_clicked() { qDebug() << "workspace settings clicked"; }
void MainChatWidget::on_newThreadButton_clicked() { qDebug() << "new thread clicked"; }
void MainChatWidget::on_assistantChatsButton_clicked() { qDebug() << "assistant chats clicked"; }
void MainChatWidget::on_bottomSettingButton_clicked() {
    qDebug() << "bottom setting clicked";
    emit bottomSettingClicked();
}
void MainChatWidget::on_headerSettingsButton_clicked() { qDebug() << "header settings clicked"; }
void MainChatWidget::on_toolsButton_clicked() { qDebug() << "tools clicked"; }
void MainChatWidget::on_micButton_clicked() { qDebug() << "mic clicked"; }
void MainChatWidget::on_sendButton_clicked() { qDebug() << "send clicked"; }
void MainChatWidget::on_createAgentButton_clicked() { qDebug() << "create agent clicked"; }
void MainChatWidget::on_editWorkspaceButton_clicked() { qDebug() << "edit workspace clicked"; }
void MainChatWidget::on_uploadFileButton_clicked() { qDebug() << "upload file clicked"; }
