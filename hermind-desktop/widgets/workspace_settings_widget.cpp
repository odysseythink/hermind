#include "workspace_settings_widget.h"
#include "workspace_settings_tab.h"
#include "sidebar_menu_button.h"
#include "icon_button.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "hermind_api_client.h"

#include <QButtonGroup>
#include <QHBoxLayout>
#include <QLabel>
#include <QStackedWidget>
#include <QVBoxLayout>

WorkspaceSettingsWidget::WorkspaceSettingsWidget(HermindApiClient *apiClient,
                                                 QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    buildUi();
    applyStyle();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &WorkspaceSettingsWidget::applyStyle);
}

QString WorkspaceSettingsWidget::workspaceSlug() const
{
    return m_workspaceSlug;
}

QString WorkspaceSettingsWidget::currentTabId() const
{
    return m_currentTabId;
}

void WorkspaceSettingsWidget::setWorkspaceSlug(const QString &slug)
{
    if (m_workspaceSlug == slug)
        return;

    m_workspaceSlug = slug;
    m_workspace = HermindWorkspace();
    loadWorkspace();
}

void WorkspaceSettingsWidget::setActiveTab(const QString &tabId)
{
    int idx = WorkspaceSettingsTabs::indexOf(tabId);
    if (idx < 0)
        idx = 0;

    const QString actualId = WorkspaceSettingsTabs::all().at(idx).id;
    if (m_currentTabId == actualId && m_contentStack->currentIndex() == idx)
        return;

    m_currentTabId = actualId;
    m_contentStack->setCurrentIndex(idx);

    SidebarMenuButton *btn = m_tabButtons.value(actualId);
    if (btn && !btn->isChecked())
        btn->setChecked(true);

    m_headerTitleLabel->setText(WorkspaceSettingsTabs::titleOf(actualId));
    emit tabChanged(actualId);
}

void WorkspaceSettingsWidget::setTabWidget(const QString &tabId, QWidget *widget)
{
    if (!widget || !m_contentStack)
        return;

    const int idx = WorkspaceSettingsTabs::indexOf(tabId);
    if (idx < 0)
        return;

    QWidget *old = m_contentStack->widget(idx);
    if (old) {
        m_contentStack->removeWidget(old);
        old->deleteLater();
    }

    widget->setParent(m_contentStack);
    m_contentStack->insertWidget(idx, widget);

    if (m_currentTabId == tabId)
        m_contentStack->setCurrentIndex(idx);
}

void WorkspaceSettingsWidget::onTabButtonClicked()
{
    auto *btn = qobject_cast<SidebarMenuButton *>(sender());
    if (!btn)
        return;
    setActiveTab(btn->property("tabId").toString());
}

void WorkspaceSettingsWidget::onWorkspacesLoaded(
    const QVector<HermindWorkspace> &workspaces,
    const ApiError &error)
{
    if (!error.isEmpty() || workspaces.isEmpty()) {
        m_workspaceNameLabel->setText(m_workspaceSlug.isEmpty()
                                          ? QString()
                                          : tr("Load failed"));
        emit workspaceLoaded(false);
        return;
    }

    for (const HermindWorkspace &ws : workspaces) {
        if (ws.slug() == m_workspaceSlug) {
            m_workspace = ws;
            m_workspaceNameLabel->setText(ws.name());
            emit workspaceLoaded(true);
            return;
        }
    }

    m_workspaceNameLabel->setText(m_workspaceSlug);
    emit workspaceLoaded(false);
}

void WorkspaceSettingsWidget::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString windowBg = ThemeColors::windowBackground(dark).name();
    const QString sidebarBg = ThemeColors::sidebarBackground(dark).name();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();

    setStyleSheet(QStringLiteral(R"(
        WorkspaceSettingsWidget {
            background-color: %1;
        }
        #settingsSidebar {
            background-color: %2;
            border: none;
        }
        #workspaceNameLabel {
            color: %3;
            font-size: 16px;
            font-weight: 600;
        }
        #headerTitleLabel {
            color: %3;
            font-size: 20px;
            font-weight: 600;
        }
        #settingsContent {
            background-color: %1;
            border: none;
        }
    )").arg(windowBg, sidebarBg, textPrimary));

    m_workspaceNameLabel->setStyleSheet(QStringLiteral(
        "QLabel { color: %1; font-size: 16px; font-weight: 600; }"
    ).arg(textPrimary));

    m_headerTitleLabel->setStyleSheet(QStringLiteral(
        "QLabel { color: %1; font-size: 20px; font-weight: 600; }"
    ).arg(textPrimary));
}

void WorkspaceSettingsWidget::buildUi()
{
    auto *rootLayout = new QHBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(0);

    // Sidebar
    auto *sidebar = new QWidget(this);
    sidebar->setObjectName(QStringLiteral("settingsSidebar"));
    sidebar->setFixedWidth(260);
    auto *sidebarLayout = new QVBoxLayout(sidebar);
    sidebarLayout->setContentsMargins(16, 16, 16, 16);
    sidebarLayout->setSpacing(12);

    m_workspaceNameLabel = new QLabel(sidebar);
    m_workspaceNameLabel->setObjectName(QStringLiteral("workspaceNameLabel"));
    m_workspaceNameLabel->setWordWrap(true);
    sidebarLayout->addWidget(m_workspaceNameLabel);

    auto *backButton = new IconButton(sidebar);
    backButton->setObjectName(QStringLiteral("returnButton"));
    backButton->setIconText(QStringLiteral("←"));
    backButton->setToolTip(tr("Return"));
    connect(backButton, &IconButton::clicked,
            this, &WorkspaceSettingsWidget::returnClicked);
    sidebarLayout->addWidget(backButton);

    sidebarLayout->addSpacing(8);

    m_tabGroup = new QButtonGroup(this);
    m_tabGroup->setExclusive(true);

    const auto &tabs = WorkspaceSettingsTabs::all();
    for (const WorkspaceSettingsTab &tab : tabs) {
        auto *btn = new SidebarMenuButton(tab.title, sidebar);
        btn->setObjectName(QStringLiteral("tabButton_") + tab.id);
        btn->setProperty("tabId", tab.id);
        btn->setCheckable(true);
        m_tabGroup->addButton(btn);
        m_tabButtons.insert(tab.id, btn);
        connect(btn, &SidebarMenuButton::clicked,
                this, &WorkspaceSettingsWidget::onTabButtonClicked);
        sidebarLayout->addWidget(btn);
    }
    sidebarLayout->addStretch();

    rootLayout->addWidget(sidebar);

    // Content
    auto *content = new QWidget(this);
    content->setObjectName(QStringLiteral("settingsContent"));
    auto *contentLayout = new QVBoxLayout(content);
    contentLayout->setContentsMargins(24, 24, 24, 24);
    contentLayout->setSpacing(16);

    m_headerTitleLabel = new QLabel(content);
    m_headerTitleLabel->setObjectName(QStringLiteral("headerTitleLabel"));
    contentLayout->addWidget(m_headerTitleLabel);

    m_contentStack = new QStackedWidget(content);
    m_contentStack->setObjectName(QStringLiteral("contentStack"));

    for (const WorkspaceSettingsTab &tab : tabs) {
        auto *page = new QWidget();
        page->setObjectName(QStringLiteral("page_") + tab.id);
        auto *pageLayout = new QVBoxLayout(page);
        pageLayout->addStretch();
        auto *placeholder = new QLabel(
            tr("%1 settings will appear here").arg(tab.title));
        placeholder->setAlignment(Qt::AlignCenter);
        pageLayout->addWidget(placeholder);
        pageLayout->addStretch();
        m_contentStack->addWidget(page);
    }

    contentLayout->addWidget(m_contentStack, 1);
    rootLayout->addWidget(content, 1);

    // Default tab
    setActiveTab(QStringLiteral("general-appearance"));
}

void WorkspaceSettingsWidget::loadWorkspace()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty()) {
        m_workspaceNameLabel->clear();
        emit workspaceLoaded(false);
        return;
    }

    m_apiClient->listWorkspaces(
        [this](const QVector<HermindWorkspace> &workspaces,
               const ApiError &error) {
            onWorkspacesLoaded(workspaces, error);
        });
}
