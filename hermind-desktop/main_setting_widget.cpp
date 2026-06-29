#include "main_setting_widget.h"
#include "ui_main_setting_widget.h"

#include <QButtonGroup>
#include <QDebug>
#include <QPixmap>

MainSettingWidget::MainSettingWidget(QWidget *parent)
    : QWidget(parent)
    , ui(new Ui::MainSettingWidget)
{
    ui->setupUi(this);

    setupLogo();
    setupMenuGroup();
    setupConnections();
    setupStyleSheet();
}

MainSettingWidget::~MainSettingWidget()
{
    delete ui;
}

void MainSettingWidget::setupLogo()
{
    QPixmap logo(":/images/logo.svg");
    if (!logo.isNull()) {
        ui->logoLabel->setPixmap(logo.scaled(24, 24, Qt::KeepAspectRatio, Qt::SmoothTransformation));
    }
}

void MainSettingWidget::setupMenuGroup()
{
    QButtonGroup *menuGroup = new QButtonGroup(this);
    menuGroup->setExclusive(true);
    menuGroup->addButton(ui->aiProviderButton);
    menuGroup->addButton(ui->adminButton);
    menuGroup->addButton(ui->agentSkillsButton);
    menuGroup->addButton(ui->meetingAssistantButton);
    menuGroup->addButton(ui->desktopAssistantButton);
    menuGroup->addButton(ui->communityCenterButton);
    menuGroup->addButton(ui->appearanceButton);
    menuGroup->addButton(ui->channelsButton);
    menuGroup->addButton(ui->toolsButton);

    ui->appearanceButton->setChecked(true);
}

void MainSettingWidget::setupConnections()
{
    connect(ui->aiProviderButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_aiProviderButton_clicked);
    connect(ui->adminButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_adminButton_clicked);
    connect(ui->agentSkillsButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_agentSkillsButton_clicked);
    connect(ui->meetingAssistantButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_meetingAssistantButton_clicked);
    connect(ui->desktopAssistantButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_desktopAssistantButton_clicked);
    connect(ui->communityCenterButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_communityCenterButton_clicked);
    connect(ui->appearanceButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_appearanceButton_clicked);
    connect(ui->channelsButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_channelsButton_clicked);
    connect(ui->toolsButton, &QPushButton::clicked,
            this, &MainSettingWidget::on_toolsButton_clicked);

    connect(ui->bottomReturnButton, &QToolButton::clicked,
            this, &MainSettingWidget::on_bottomReturnButton_clicked);

    connect(ui->defaultWindowCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &MainSettingWidget::on_defaultWindowCombo_currentIndexChanged);
    connect(ui->themeCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &MainSettingWidget::on_themeCombo_currentIndexChanged);
    connect(ui->languageCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &MainSettingWidget::on_languageCombo_currentIndexChanged);
}

void MainSettingWidget::setupStyleSheet()
{
    setStyleSheet(R"(
        MainSettingWidget {
            background-color: #FFFFFF;
        }

        #sidebarFrame {
            background-color: #E8EDF2;
            border: none;
        }

        #brandLabel {
            color: #1F2937;
            font-size: 16px;
            font-weight: 600;
        }

        #settingsSectionLabel {
            color: #6B7280;
            font-size: 12px;
            font-weight: 500;
            margin-top: 8px;
        }

        #sidebarFrame QPushButton {
            text-align: left;
            border: none;
            border-radius: 8px;
            background-color: transparent;
            color: #374151;
            font-size: 14px;
            padding: 10px 12px;
        }

        #sidebarFrame QPushButton:hover {
            background-color: #DDE3E9;
        }

        #sidebarFrame QPushButton:checked {
            background-color: #D6E8F7;
            color: #1F2937;
        }

        #supportLabel, #privacyLabel, #termsLabel {
            color: #6B7280;
            font-size: 12px;
            padding: 2px 12px;
        }

        #bottomChatButton, #bottomDocsButton, #bottomGithubButton, #bottomReturnButton {
            border: none;
            border-radius: 14px;
            background-color: #DFE5EA;
            color: #4B5563;
            min-width: 28px;
            max-width: 28px;
            min-height: 28px;
            max-height: 28px;
            padding: 0px;
        }

        #bottomChatButton:hover, #bottomDocsButton:hover, #bottomGithubButton:hover, #bottomReturnButton:hover {
            background-color: #D2D8DE;
        }

        #contentFrame {
            background-color: #FFFFFF;
            border: 1px solid #E5E7EB;
            border-radius: 16px;
        }

        #titleLabel {
            color: #1F2937;
            font-size: 22px;
            font-weight: 600;
        }

        #subtitleLabel {
            color: #6B7280;
            font-size: 13px;
        }

        #separatorLine {
            color: #E5E7EB;
            background-color: #E5E7EB;
            border: none;
            max-height: 1px;
        }

        #defaultWindowLabel, #themeLabel, #languageLabel {
            color: #1F2937;
            font-size: 14px;
            font-weight: 500;
        }

        #defaultWindowDesc, #themeDesc, #languageDesc {
            color: #6B7280;
            font-size: 12px;
        }

        QComboBox {
            background-color: #F1F3F5;
            border: 1px solid #E5E7EB;
            border-radius: 8px;
            padding: 8px 12px;
            color: #1F2937;
            font-size: 13px;
            min-width: 140px;
            max-width: 220px;
        }

        QComboBox:hover {
            border-color: #D1D5DB;
        }

        QComboBox::drop-down {
            border: none;
            width: 24px;
        }

        QComboBox QAbstractItemView {
            background-color: #FFFFFF;
            border: 1px solid #E5E7EB;
            selection-background-color: #D6E8F7;
        }
    )");
}

void MainSettingWidget::on_aiProviderButton_clicked() { qDebug() << "ai provider clicked"; }
void MainSettingWidget::on_adminButton_clicked() { qDebug() << "admin clicked"; }
void MainSettingWidget::on_agentSkillsButton_clicked() { qDebug() << "agent skills clicked"; }
void MainSettingWidget::on_meetingAssistantButton_clicked() { qDebug() << "meeting assistant clicked"; }
void MainSettingWidget::on_desktopAssistantButton_clicked() { qDebug() << "desktop assistant clicked"; }
void MainSettingWidget::on_communityCenterButton_clicked() { qDebug() << "community center clicked"; }
void MainSettingWidget::on_appearanceButton_clicked() { qDebug() << "appearance clicked"; }
void MainSettingWidget::on_channelsButton_clicked() { qDebug() << "channels clicked"; }
void MainSettingWidget::on_toolsButton_clicked() { qDebug() << "tools clicked"; }

void MainSettingWidget::on_bottomReturnButton_clicked() {
    qDebug() << "bottom return clicked";
    emit bottomReturnClicked();
}

void MainSettingWidget::on_defaultWindowCombo_currentIndexChanged(int index)
{
    qDebug() << "default window changed:" << index;
}

void MainSettingWidget::on_themeCombo_currentIndexChanged(int index)
{
    qDebug() << "theme changed:" << index;
}

void MainSettingWidget::on_languageCombo_currentIndexChanged(int index)
{
    qDebug() << "language changed:" << index;
}
