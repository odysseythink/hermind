#include <QtTest/QtTest>
#include <QSignalSpy>
#include <QLabel>
#include <QStackedWidget>

#include "ai_provider_settings_widget.h"
#include "ai_provider_settings_tab.h"
#include "sidebar_menu_button.h"

class TestAiProviderSettingsWidget : public QObject
{
    Q_OBJECT
private slots:
    void tabButtonsExistForAllSevenTabs();
    void defaultTabIsLlmPreference();
    void setActiveTabSwitchesStackAndEmitsTabChanged();
    void unknownTabFallsBackToFirst();
    void setTabWidgetReplacesPlaceholder();
    void buttonsVisibleByDefaultForSingleUser();
    void buttonsHiddenForMemberRole();
    void buttonsVisibleForAdminRole();
    void activeTabFallsBackWhenRoleRevoked();
};

void TestAiProviderSettingsWidget::tabButtonsExistForAllSevenTabs()
{
    AiProviderSettingsWidget w(nullptr);
    for (const AiProviderSettingsTab &tab : AiProviderSettingsTabs::all()) {
        auto *btn = w.findChild<SidebarMenuButton *>(
            QStringLiteral("tabButton_") + tab.id);
        QVERIFY2(btn, qPrintable(tab.id));
    }
}

void TestAiProviderSettingsWidget::defaultTabIsLlmPreference()
{
    AiProviderSettingsWidget w(nullptr);
    QCOMPARE(w.currentTabId(), QStringLiteral("llm-preference"));
}

void TestAiProviderSettingsWidget::setActiveTabSwitchesStackAndEmitsTabChanged()
{
    AiProviderSettingsWidget w(nullptr);
    QSignalSpy spy(&w, &AiProviderSettingsWidget::tabChanged);
    w.setActiveTab(QStringLiteral("audio-preference"));
    QCOMPARE(w.currentTabId(), QStringLiteral("audio-preference"));
    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.takeFirst().at(0).toString(), QStringLiteral("audio-preference"));

    auto *stack = w.findChild<QStackedWidget *>(QStringLiteral("contentStack"));
    QVERIFY(stack);
    QCOMPARE(stack->currentIndex(),
             AiProviderSettingsTabs::indexOf(QStringLiteral("audio-preference")));

    auto *btn = w.findChild<SidebarMenuButton *>(
        QStringLiteral("tabButton_audio-preference"));
    QVERIFY(btn && btn->isChecked());
}

void TestAiProviderSettingsWidget::unknownTabFallsBackToFirst()
{
    AiProviderSettingsWidget w(nullptr);
    w.setActiveTab(QStringLiteral("does-not-exist"));
    QCOMPARE(w.currentTabId(), QStringLiteral("llm-preference"));
}

void TestAiProviderSettingsWidget::setTabWidgetReplacesPlaceholder()
{
    AiProviderSettingsWidget w(nullptr);
    auto *page = new QLabel(QStringLiteral("native page"));
    page->setObjectName(QStringLiteral("injectedPage"));
    w.setTabWidget(QStringLiteral("audio-preference"), page);
    w.setActiveTab(QStringLiteral("audio-preference"));

    auto *stack = w.findChild<QStackedWidget *>(QStringLiteral("contentStack"));
    QVERIFY(stack);
    QCOMPARE(stack->currentWidget(), static_cast<QWidget *>(page));
}

void TestAiProviderSettingsWidget::buttonsVisibleByDefaultForSingleUser()
{
    // 单机模式：AuthManager 用户为空（role 为空字符串），AI 提供商页完全可见。
    AiProviderSettingsWidget w(nullptr);
    auto *btn = w.findChild<SidebarMenuButton *>(
        QStringLiteral("tabButton_llm-preference"));
    QVERIFY(btn);
    QVERIFY(!btn->isHidden());
}

void TestAiProviderSettingsWidget::buttonsHiddenForMemberRole()
{
    AiProviderSettingsWidget w(nullptr);
    w.setUserRole(QStringLiteral("member"));
    auto *btn = w.findChild<SidebarMenuButton *>(
        QStringLiteral("tabButton_llm-preference"));
    QVERIFY(btn);
    QVERIFY(btn->isHidden());
}

void TestAiProviderSettingsWidget::buttonsVisibleForAdminRole()
{
    AiProviderSettingsWidget w(nullptr);
    w.setUserRole(QStringLiteral("member"));
    w.setUserRole(QStringLiteral("admin"));
    auto *btn = w.findChild<SidebarMenuButton *>(
        QStringLiteral("tabButton_llm-preference"));
    QVERIFY(btn);
    QVERIFY(!btn->isHidden());
}

void TestAiProviderSettingsWidget::activeTabFallsBackWhenRoleRevoked()
{
    AiProviderSettingsWidget w(nullptr);
    w.setActiveTab(QStringLiteral("model-routers"));
    QCOMPARE(w.currentTabId(), QStringLiteral("model-routers"));
    w.setUserRole(QStringLiteral("member"));
    QCOMPARE(w.currentTabId(), QStringLiteral("llm-preference"));
}

QTEST_MAIN(TestAiProviderSettingsWidget)
#include "tst_ai_provider_settings_widget.moc"
