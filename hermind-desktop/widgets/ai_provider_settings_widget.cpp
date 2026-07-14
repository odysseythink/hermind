#include "ai_provider_settings_widget.h"
#include "ai_provider_settings_tab.h"
#include "sidebar_menu_button.h"
#include "icon_button.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include "auth_manager.h"

#include <QButtonGroup>
#include <QHBoxLayout>
#include <QLabel>
#include <QStackedWidget>
#include <QVBoxLayout>

AiProviderSettingsWidget::AiProviderSettingsWidget(HermindApiClient *apiClient,
                                                   QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    buildUi();
    applyStyle();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &AiProviderSettingsWidget::applyStyle);
    connect(&AuthManager::instance(), &AuthManager::userChanged,
            this, [this](const HermindUser &user) {
                setUserRole(user.role());
            });
    setUserRole(AuthManager::instance().currentUser().role());
}

QString AiProviderSettingsWidget::currentTabId() const
{
    return m_currentTabId;
}

void AiProviderSettingsWidget::setActiveTab(const QString &tabId)
{
    int idx = AiProviderSettingsTabs::indexOf(tabId);
    if (idx < 0)
        idx = 0;

    const QString actualId = AiProviderSettingsTabs::all().at(idx).id;
    if (m_currentTabId == actualId && m_contentStack->currentIndex() == idx)
        return;

    m_currentTabId = actualId;
    m_contentStack->setCurrentIndex(idx);

    SidebarMenuButton *btn = m_tabButtons.value(actualId);
    if (btn && !btn->isChecked())
        btn->setChecked(true);

    m_headerTitleLabel->setText(AiProviderSettingsTabs::titleOf(actualId));

    emit tabChanged(actualId);
}

void AiProviderSettingsWidget::setUserRole(const QString &role)
{
    // Single-user mode yields an empty role (AuthManager keeps an empty
    // user); web treats "no user" as full access. Any non-empty role
    // other than admin is denied (web: roles: ["admin"]).
    const bool isAdmin = role.isEmpty() || role == QStringLiteral("admin");

    for (SidebarMenuButton *btn : m_tabButtons)
        btn->setVisible(isAdmin);

    if (!isAdmin && m_currentTabId != QStringLiteral("llm-preference"))
        setActiveTab(QStringLiteral("llm-preference"));
}

void AiProviderSettingsWidget::setTabWidget(const QString &tabId, QWidget *widget)
{
    if (!widget || !m_contentStack)
        return;

    const int idx = AiProviderSettingsTabs::indexOf(tabId);
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

void AiProviderSettingsWidget::onTabButtonClicked()
{
    auto *btn = qobject_cast<SidebarMenuButton *>(sender());
    if (!btn)
        return;
    setActiveTab(btn->property("tabId").toString());
}

void AiProviderSettingsWidget::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString windowBg = ThemeColors::windowBackground(dark).name();
    const QString sidebarBg = ThemeColors::sidebarBackground(dark).name();
    const QString textPrimary = ThemeColors::textPrimary(dark).name();

    setStyleSheet(QStringLiteral(R"(
        AiProviderSettingsWidget {
            background-color: %1;
        }
        #aiProviderSidebar {
            background-color: %2;
            border: none;
        }
        #aiProviderHeaderLabel {
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

    m_headerTitleLabel->setStyleSheet(QStringLiteral(
        "QLabel { color: %1; font-size: 20px; font-weight: 600; }"
    ).arg(textPrimary));
}

void AiProviderSettingsWidget::buildUi()
{
    auto *rootLayout = new QHBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(0);

    // Sidebar
    auto *sidebar = new QWidget(this);
    sidebar->setObjectName(QStringLiteral("aiProviderSidebar"));
    sidebar->setFixedWidth(260);
    auto *sidebarLayout = new QVBoxLayout(sidebar);
    sidebarLayout->setContentsMargins(16, 16, 16, 16);
    sidebarLayout->setSpacing(12);

    auto *headerLabel = new QLabel(tr("AI Providers"), sidebar);
    headerLabel->setObjectName(QStringLiteral("aiProviderHeaderLabel"));
    headerLabel->setWordWrap(true);
    sidebarLayout->addWidget(headerLabel);

    auto *backButton = new IconButton(sidebar);
    backButton->setObjectName(QStringLiteral("returnButton"));
    backButton->setIconText(QStringLiteral("←"));
    backButton->setToolTip(tr("Return"));
    connect(backButton, &IconButton::clicked,
            this, &AiProviderSettingsWidget::returnClicked);
    sidebarLayout->addWidget(backButton);

    sidebarLayout->addSpacing(8);

    m_tabGroup = new QButtonGroup(this);
    m_tabGroup->setExclusive(true);

    const auto &tabs = AiProviderSettingsTabs::all();
    for (const AiProviderSettingsTab &tab : tabs) {
        auto *btn = new SidebarMenuButton(tab.title, sidebar);
        btn->setObjectName(QStringLiteral("tabButton_") + tab.id);
        btn->setProperty("tabId", tab.id);
        btn->setCheckable(true);
        m_tabGroup->addButton(btn);
        m_tabButtons.insert(tab.id, btn);
        connect(btn, &SidebarMenuButton::clicked,
                this, &AiProviderSettingsWidget::onTabButtonClicked);
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

    for (const AiProviderSettingsTab &tab : tabs) {
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

    setActiveTab(QStringLiteral("llm-preference"));
}
