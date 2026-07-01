#include "main_setting_widget.h"
#include "ui_main_setting_widget.h"
#include "theme_manager.h"
#include "widgets/sidebar_menu_button.h"
#include "widgets/styled_separator.h"
#include "widgets/rounded_frame.h"
#include "widgets/setting_row.h"
#include "widgets/theme_colors.h"

#include <QButtonGroup>
#include <QDebug>
#include <QPixmap>
#include <QPushButton>
#include <QComboBox>
#include <QFrame>
#include <QVBoxLayout>
#include <QBoxLayout>

MainSettingWidget::MainSettingWidget(QWidget *parent)
    : QWidget(parent)
    , ui(new Ui::MainSettingWidget)
{
    ui->setupUi(this);

    replaceMenuButtons();
    replaceSeparator();
    replaceContentFrame();
    rebuildSettingRows();

    setupLogo();
    setupMenuGroup();
    setupConnections();
    setupStyleSheet();
}

MainSettingWidget::~MainSettingWidget()
{
    delete ui;
}

void MainSettingWidget::replaceMenuButtons()
{
    auto replaceOne = [this](const QString &name) {
        QPushButton *oldBtn = findChild<QPushButton *>(name);
        if (!oldBtn)
            return;

        SidebarMenuButton *newBtn = new SidebarMenuButton(oldBtn->text(), this);
        newBtn->setObjectName(name);
        newBtn->setChecked(oldBtn->isChecked());

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

        // Update ui-> pointer
        if (name == QLatin1String("aiProviderButton")) ui->aiProviderButton = newBtn;
        else if (name == QLatin1String("adminButton")) ui->adminButton = newBtn;
        else if (name == QLatin1String("agentSkillsButton")) ui->agentSkillsButton = newBtn;
        else if (name == QLatin1String("meetingAssistantButton")) ui->meetingAssistantButton = newBtn;
        else if (name == QLatin1String("desktopAssistantButton")) ui->desktopAssistantButton = newBtn;
        else if (name == QLatin1String("communityCenterButton")) ui->communityCenterButton = newBtn;
        else if (name == QLatin1String("appearanceButton")) ui->appearanceButton = newBtn;
        else if (name == QLatin1String("channelsButton")) ui->channelsButton = newBtn;
        else if (name == QLatin1String("toolsButton")) ui->toolsButton = newBtn;
    };

    replaceOne(QStringLiteral("aiProviderButton"));
    replaceOne(QStringLiteral("adminButton"));
    replaceOne(QStringLiteral("agentSkillsButton"));
    replaceOne(QStringLiteral("meetingAssistantButton"));
    replaceOne(QStringLiteral("desktopAssistantButton"));
    replaceOne(QStringLiteral("communityCenterButton"));
    replaceOne(QStringLiteral("appearanceButton"));
    replaceOne(QStringLiteral("channelsButton"));
    replaceOne(QStringLiteral("toolsButton"));
}

void MainSettingWidget::replaceSeparator()
{
    QFrame *oldSep = ui->separatorLine;
    if (!oldSep)
        return;

    StyledSeparator *newSep = new StyledSeparator(this);
    newSep->setObjectName(QStringLiteral("separatorLine"));

    QLayout *layout = oldSep->parentWidget()->layout();
    if (layout) {
        int idx = -1;
        for (int i = 0; i < layout->count(); ++i) {
            if (layout->itemAt(i)->widget() == oldSep) {
                idx = i;
                break;
            }
        }
        if (idx >= 0) {
            layout->removeWidget(oldSep);
            if (auto *box = qobject_cast<QBoxLayout *>(layout))
                box->insertWidget(idx, newSep);
        }
    }

    oldSep->deleteLater();
    ui->separatorLine = newSep;
}

void MainSettingWidget::replaceContentFrame()
{
    QFrame *oldFrame = ui->contentFrame;
    if (!oldFrame)
        return;

    RoundedFrame *newFrame = new RoundedFrame(this);
    newFrame->setObjectName(QStringLiteral("contentFrame"));
    newFrame->setRadius(16);

    // Move children from old frame to new frame
    QLayout *oldLayout = oldFrame->layout();
    auto *newLayout = new QVBoxLayout(newFrame);
    if (oldLayout) {
        newLayout->setSpacing(oldLayout->spacing());
        newLayout->setContentsMargins(oldLayout->contentsMargins());

        while (oldLayout->count() > 0) {
            QLayoutItem *item = oldLayout->takeAt(0);
            if (item->widget())
                newLayout->addWidget(item->widget());
            delete item;
        }
    } else {
        newLayout->setSpacing(24);
        newLayout->setContentsMargins(32, 32, 32, 32);
    }
    newLayout->addStretch();

    QLayout *parentLayout = oldFrame->parentWidget()->layout();
    if (parentLayout) {
        int idx = -1;
        for (int i = 0; i < parentLayout->count(); ++i) {
            if (parentLayout->itemAt(i)->widget() == oldFrame) {
                idx = i;
                break;
            }
        }
        if (idx >= 0) {
            parentLayout->removeWidget(oldFrame);
            if (auto *box = qobject_cast<QBoxLayout *>(parentLayout))
                box->insertWidget(idx, newFrame);
        }
    }

    oldFrame->deleteLater();
    ui->contentFrame = newFrame;
}

void MainSettingWidget::rebuildSettingRows()
{
    struct RowSpec {
        QString rowName;
        QString title;
        QString desc;
        QString comboName;
    };

    const QList<RowSpec> rows = {
        { QStringLiteral("defaultWindowRow"), tr("默认窗口"), tr("设置应用程序启动时默认显示的窗口。"), QStringLiteral("defaultWindowCombo") },
        { QStringLiteral("themeRow"), tr("主题"), tr("选择您偏好的应用配色主题。"), QStringLiteral("themeCombo") },
        { QStringLiteral("languageRow"), tr("显示语言"), tr("选择显示 AnythingLLM 界面所用的语言（若有翻译可用）。"), QStringLiteral("languageCombo") }
    };

    for (const RowSpec &spec : rows) {
        QFrame *rowFrame = findChild<QFrame *>(spec.rowName);
        QComboBox *combo = findChild<QComboBox *>(spec.comboName);
        if (!rowFrame || !combo)
            continue;

        SettingRow *settingRow = new SettingRow(this);
        settingRow->setObjectName(spec.rowName);
        settingRow->setTitle(spec.title);
        settingRow->setDescription(spec.desc);
        settingRow->setControl(combo);

        QLayout *parentLayout = rowFrame->parentWidget()->layout();
        if (parentLayout) {
            int idx = -1;
            for (int i = 0; i < parentLayout->count(); ++i) {
                if (parentLayout->itemAt(i)->widget() == rowFrame) {
                    idx = i;
                    break;
                }
            }
            if (idx >= 0) {
                parentLayout->removeWidget(rowFrame);
                if (auto *box = qobject_cast<QBoxLayout *>(parentLayout))
                    box->insertWidget(idx, settingRow);
            }
        }

        rowFrame->deleteLater();
    }
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
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString windowBg = ThemeColors::windowBackground(dark).name();
    const QString sidebarBg = ThemeColors::sidebarBackground(dark).name();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();
    const QString textSecondary = ThemeColors::textSecondary(dark).name();
    const QString selectedBg = ThemeColors::selectedBackground(dark).name();

    setStyleSheet(QStringLiteral(R"(
        MainSettingWidget {
            background-color: %1;
        }

        #sidebarFrame {
            background-color: %2;
            border: none;
        }

        #brandLabel {
            color: %3;
            font-size: 16px;
            font-weight: 600;
        }

        #settingsSectionLabel {
            color: %4;
            font-size: 12px;
            font-weight: 500;
            margin-top: 8px;
        }

        #supportLabel, #privacyLabel, #termsLabel {
            color: %4;
            font-size: 12px;
            padding: 2px 12px;
        }

        #bottomReturnButton {
            border: none;
            border-radius: 14px;
            background-color: %2;
            color: %4;
            min-width: 28px;
            max-width: 28px;
            min-height: 28px;
            max-height: 28px;
            padding: 0px;
        }

        #bottomReturnButton:hover {
            background-color: %5;
        }

        #titleLabel {
            color: %3;
            font-size: 22px;
            font-weight: 600;
        }

        #subtitleLabel {
            color: %4;
            font-size: 13px;
        }

        QComboBox {
            background-color: %2;
            border: 1px solid %5;
            border-radius: 8px;
            padding: 8px 12px;
            color: %3;
            font-size: 13px;
            min-width: 140px;
            max-width: 220px;
        }

        QComboBox:hover {
            border-color: %4;
        }

        QComboBox::drop-down {
            border: none;
            width: 24px;
        }

        QComboBox QAbstractItemView {
            background-color: %1;
            border: 1px solid %5;
            selection-background-color: %6;
        }
    )").arg(windowBg, sidebarBg, textPrimary, textSecondary,
            ThemeColors::border(dark).name(), selectedBg));
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
    switch (index) {
    case 0: ThemeManager::instance().setTheme(QStringLiteral("system")); break;
    case 1: ThemeManager::instance().setTheme(QStringLiteral("light"));  break;
    case 2: ThemeManager::instance().setTheme(QStringLiteral("dark"));   break;
    }
    qDebug() << "theme changed:" << index << "→" << ThemeManager::instance().theme();
}

void MainSettingWidget::on_languageCombo_currentIndexChanged(int index)
{
    switch (index) {
    case 0: ThemeManager::instance().setLanguage(QStringLiteral("en")); break;
    case 1: ThemeManager::instance().setLanguage(QStringLiteral("zh")); break;
    }
    qDebug() << "language changed:" << index << "→" << ThemeManager::instance().language();
}
