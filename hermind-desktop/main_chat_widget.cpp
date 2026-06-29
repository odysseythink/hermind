#include "main_chat_widget.h"
#include "ui_main_chat_widget.h"

MainChatWidget::MainChatWidget(QWidget *parent)
    : QWidget(parent)
    , ui(new Ui::MainChatWidget)
{
    ui->setupUi(this);


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
    setStyleSheet(R"(
        QWidget {
            background-color: #FFFFFF;
        }

        #sidebarFrame {
            background-color: #E8EDF2;
            border: none;
        }

        #chatFrame {
            background-color: #FFFFFF;
            border: none;
        }

        #brandLabel {
            color: #1F2937;
            font-size: 16px;
            font-weight: 600;
        }

        #workspaceLabel {
            color: #1F2937;
            font-size: 13px;
            font-weight: 500;
        }

        #workspaceIcon {
            color: #6B7280;
            font-size: 12px;
        }

        QToolButton {
            border: none;
            border-radius: 6px;
            background-color: transparent;
            color: #4B5563;
            font-size: 14px;
            padding: 4px 6px;
        }

        QToolButton:hover {
            background-color: #DDE3E9;
        }

        #popoutButton, #newSearchButton, #uploadButton, #workspaceSettingsButton,
        #headerSettingsButton, #micButton, #bottomChatButton, #bottomDocsButton,
        #bottomGithubButton, #bottomSettingButton {
            min-width: 28px;
            max-width: 28px;
            min-height: 28px;
            max-height: 28px;
            padding: 0px;
        }

        #sendButton {
            background-color: #5B8DEF;
            color: #FFFFFF;
            border-radius: 14px;
            min-width: 28px;
            max-width: 28px;
            min-height: 28px;
            max-height: 28px;
            padding: 0px;
        }

        #sendButton:hover {
            background-color: #4A7DE0;
        }

        #searchEdit {
            background-color: #FFFFFF;
            border: 1px solid #DDE3E9;
            border-radius: 8px;
            padding: 6px 10px;
            color: #1F2937;
            font-size: 13px;
        }

        #searchEdit::placeholder {
            color: #9CA3AF;
        }

        #threadList {
            background-color: transparent;
            border: none;
            outline: none;
        }

        #threadList::item {
            color: #374151;
            font-size: 13px;
            padding: 8px 12px;
            border-radius: 6px;
        }

        #threadList::item:selected {
            background-color: #D6E8F7;
            color: #1F2937;
        }

        #threadList::item:hover {
            background-color: #DFE5EA;
        }

        #newThreadButton, #assistantChatsButton {
            background-color: #FFFFFF;
            border: 1px solid #DDE3E9;
            border-radius: 8px;
            padding: 8px 12px;
            color: #1F2937;
            font-size: 13px;
            text-align: left;
        }

        #newThreadButton:hover, #assistantChatsButton:hover {
            background-color: #F8FAFC;
        }

        #workspaceNameLabel {
            color: #6B7280;
            font-size: 14px;
            font-weight: 500;
        }

        #versionLabel {
            color: #D4A017;
            font-size: 12px;
            font-weight: 500;
        }

        #welcomeLabel {
            color: #1F2937;
            font-size: 28px;
            font-weight: 500;
        }

        #inputFrame {
            background-color: #FFFFFF;
            border: 1px solid #E0E4E8;
            border-radius: 16px;
        }

        #messageEdit {
            background-color: transparent;
            border: none;
            color: #1F2937;
            font-size: 14px;
        }

        #messageEdit::placeholder {
            color: #9CA3AF;
        }

        #toolsButton {
            background-color: transparent;
            border: none;
            color: #6B7280;
            font-size: 13px;
            padding: 4px 8px;
        }

        #toolsButton:hover {
            background-color: #F1F3F5;
            border-radius: 6px;
        }

        #createAgentButton, #editWorkspaceButton, #uploadFileButton {
            background-color: #F1F3F5;
            border: none;
            border-radius: 16px;
            padding: 8px 16px;
            color: #374151;
            font-size: 13px;
        }

        #createAgentButton:hover, #editWorkspaceButton:hover, #uploadFileButton:hover {
            background-color: #E5E7EB;
        }

        #bottomChatButton, #bottomDocsButton, #bottomGithubButton, #bottomSettingButton {
            background-color: #DFE5EA;
            border-radius: 14px;
        }

        #bottomChatButton:hover, #bottomDocsButton:hover, #bottomGithubButton:hover, #bottomSettingButton:hover {
            background-color: #D2D8DE;
        }
    )");
}

void MainChatWidget::on_popoutButton_clicked() { qDebug() << "popout clicked"; }
void MainChatWidget::on_newSearchButton_clicked() { qDebug() << "new search clicked"; }
void MainChatWidget::on_uploadButton_clicked() { qDebug() << "upload clicked"; }
void MainChatWidget::on_workspaceSettingsButton_clicked() { qDebug() << "workspace settings clicked"; }
void MainChatWidget::on_newThreadButton_clicked() { qDebug() << "new thread clicked"; }
void MainChatWidget::on_assistantChatsButton_clicked() { qDebug() << "assistant chats clicked"; }
void MainChatWidget::on_bottomChatButton_clicked() { qDebug() << "bottom chat clicked"; }
void MainChatWidget::on_bottomDocsButton_clicked() { qDebug() << "bottom docs clicked"; }
void MainChatWidget::on_bottomGithubButton_clicked() { qDebug() << "bottom github clicked"; }
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