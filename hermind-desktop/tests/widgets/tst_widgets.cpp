#include <QtTest>
#include <QApplication>
#include <QComboBox>
#include "tst_llm_provider_info.h"
#include "tst_llm_model_selector.h"
#include "tst_agent_config_state.h"
#include "tst_agent_config_tab.h"
#include "icon_button.h"
#include "sidebar_menu_button.h"
#include "search_input.h"
#include "styled_separator.h"
#include "rounded_frame.h"
#include "setting_row.h"
#include "llm_provider_info.h"
#include "workspace_settings_tab.h"
#include "suggested_messages_editor.h"

class TestWidgets : public QObject
{
    Q_OBJECT

private slots:
    void iconButtonHasFixedSize();
    void sidebarMenuButtonIsCheckable();
    void sidebarMenuButtonIsChecked();
    void searchInputHasPlaceholder();
    void searchInputAcceptsText();
    void styledSeparatorHasFixedHeight();
    void styledSeparatorIsHorizontal();
    void roundedFrameHasDefaultRadius();
    void roundedFrameRadiusCanChange();
    void settingRowHoldsTitleAndDescription();
    void settingRowCanEmbedControl();
    void llmProviderDefaultExists();
    void llmProviderOpenAiSupportsModelSelection();
    void llmProviderByIdReturnsNullForUnknown();
    void workspaceSettingsTabs_all_hasFiveTabs();
    void workspaceSettingsTabs_indexOfFindsTab();
    void workspaceSettingsTabs_indexOfInvalidReturnsNegativeOne();
    void workspaceSettingsTabs_titleOfFindsTitle();
    void suggestedMessagesEditor_setMessagesPopulatesRows();
    void suggestedMessagesEditor_cannotExceedFourMessages();
    void suggestedMessagesEditor_removeMessageReducesCount();
    void suggestedMessagesEditor_validMessagesFiltersEmpty();
    void suggestedMessagesEditor_markSavedClearsChanges();
};

void TestWidgets::iconButtonHasFixedSize()
{
    IconButton btn;
    QCOMPARE(btn.minimumSize(), QSize(28, 28));
    QCOMPARE(btn.maximumSize(), QSize(28, 28));
}

void TestWidgets::sidebarMenuButtonIsCheckable()
{
    SidebarMenuButton btn(QStringLiteral("Appearance"));
    QVERIFY(btn.isCheckable());
    QVERIFY(btn.isFlat());
}

void TestWidgets::sidebarMenuButtonIsChecked()
{
    SidebarMenuButton btn(QStringLiteral("Appearance"));
    btn.setChecked(true);
    QVERIFY(btn.isChecked());
}

void TestWidgets::searchInputHasPlaceholder()
{
    SearchInput input;
    input.setPlaceholderText(QStringLiteral("Search"));
    QCOMPARE(input.placeholderText(), QStringLiteral("Search"));
}

void TestWidgets::searchInputAcceptsText()
{
    SearchInput input;
    input.setText(QStringLiteral("hello"));
    QCOMPARE(input.text(), QStringLiteral("hello"));
}

void TestWidgets::styledSeparatorHasFixedHeight()
{
    StyledSeparator sep;
    QCOMPARE(sep.minimumHeight(), 1);
    QCOMPARE(sep.maximumHeight(), 1);
}

void TestWidgets::styledSeparatorIsHorizontal()
{
    StyledSeparator sep;
    QCOMPARE(sep.frameShape(), QFrame::HLine);
}

void TestWidgets::roundedFrameHasDefaultRadius()
{
    RoundedFrame frame;
    QVERIFY(frame.styleSheet().contains(QStringLiteral("border-radius: 16px")));
}

void TestWidgets::roundedFrameRadiusCanChange()
{
    RoundedFrame frame;
    frame.setRadius(8);
    QVERIFY(frame.styleSheet().contains(QStringLiteral("border-radius: 8px")));
}

void TestWidgets::settingRowHoldsTitleAndDescription()
{
    SettingRow row;
    row.setTitle(QStringLiteral("Theme"));
    row.setDescription(QStringLiteral("Pick a theme."));
    QVERIFY(!row.findChildren<QLabel *>().isEmpty());
}

void TestWidgets::settingRowCanEmbedControl()
{
    SettingRow row;
    auto *combo = new QComboBox(&row);
    row.setControl(combo);
    QVERIFY(row.findChild<QComboBox *>() == combo);
}

void TestWidgets::llmProviderDefaultExists()
{
    const LlmProviderInfo *p = LlmProviderInfo::byId(QStringLiteral("default"));
    QVERIFY(p);
    QCOMPARE(p->name, QStringLiteral("System default"));
}

void TestWidgets::llmProviderOpenAiSupportsModelSelection()
{
    const LlmProviderInfo *p = LlmProviderInfo::byId(QStringLiteral("openai"));
    QVERIFY(p);
    QVERIFY(p->supportsModelSelection);
}

void TestWidgets::llmProviderByIdReturnsNullForUnknown()
{
    QVERIFY(!LlmProviderInfo::byId(QStringLiteral("zzzzz")));
}

void TestWidgets::workspaceSettingsTabs_all_hasFiveTabs()
{
    QCOMPARE(WorkspaceSettingsTabs::all().size(), 5);
}

void TestWidgets::workspaceSettingsTabs_indexOfFindsTab()
{
    QCOMPARE(WorkspaceSettingsTabs::indexOf(QStringLiteral("vector-database")), 2);
    QCOMPARE(WorkspaceSettingsTabs::indexOf(QStringLiteral("members")), 3);
}

void TestWidgets::workspaceSettingsTabs_indexOfInvalidReturnsNegativeOne()
{
    QCOMPARE(WorkspaceSettingsTabs::indexOf(QStringLiteral("not-a-tab")), -1);
}

void TestWidgets::workspaceSettingsTabs_titleOfFindsTitle()
{
    QVERIFY(WorkspaceSettingsTabs::titleOf(QStringLiteral("chat")).contains(
                QStringLiteral("Chat"), Qt::CaseInsensitive));
}

void TestWidgets::suggestedMessagesEditor_setMessagesPopulatesRows()
{
    SuggestedMessagesEditor editor;
    editor.setMessages(QStringList()
                       << QStringLiteral("Hello")
                       << QStringLiteral("World"));
    QCOMPARE(editor.findChildren<QLineEdit *>().size(), 2);
}

void TestWidgets::suggestedMessagesEditor_cannotExceedFourMessages()
{
    SuggestedMessagesEditor editor;
    editor.setMessages(QStringList() << "1" << "2" << "3" << "4");
    QPushButton *addBtn = editor.findChild<QPushButton *>(QStringLiteral("addMessageButton"));
    QVERIFY(addBtn);
    QVERIFY(addBtn->isEnabled());

    QTest::mouseClick(addBtn, Qt::LeftButton);
    QCOMPARE(editor.findChildren<QLineEdit *>().size(), 4);
    QVERIFY(!addBtn->isEnabled());
}

void TestWidgets::suggestedMessagesEditor_removeMessageReducesCount()
{
    SuggestedMessagesEditor editor;
    editor.setMessages(QStringList() << "a" << "b" << "c");
    QList<QAbstractButton *> removeBtns = editor.findChildren<QAbstractButton *>(
        QStringLiteral("removeMessageButton"));
    QCOMPARE(removeBtns.size(), 3);

    QTest::mouseClick(removeBtns.first(), Qt::LeftButton);
    QCOMPARE(editor.findChildren<QLineEdit *>().size(), 2);
}

void TestWidgets::suggestedMessagesEditor_validMessagesFiltersEmpty()
{
    SuggestedMessagesEditor editor;
    editor.setMessages(QStringList() << "keep" << "" << "   " << "also keep");
    const QStringList valid = editor.validMessages();
    QCOMPARE(valid.size(), 2);
    QVERIFY(valid.contains(QStringLiteral("keep")));
    QVERIFY(valid.contains(QStringLiteral("also keep")));
}

void TestWidgets::suggestedMessagesEditor_markSavedClearsChanges()
{
    SuggestedMessagesEditor editor;
    editor.setMessages(QStringList() << "old");
    QSignalSpy spy(&editor, &SuggestedMessagesEditor::saveRequested);

    QList<QLineEdit *> edits = editor.findChildren<QLineEdit *>();
    QVERIFY(!edits.isEmpty());
    edits.first()->setText(QStringLiteral("new"));
    QVERIFY(editor.hasChanges());

    editor.markSaved();
    QVERIFY(!editor.hasChanges());
}

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    Q_UNUSED(app)

    int status = 0;

    {
        TestWidgets tc;
        status |= QTest::qExec(&tc, argc, argv);
    }

    {
        TestLlmProviderInfo tc;
        status |= QTest::qExec(&tc, argc, argv);
    }

    {
        TestLlmModelSelector tc;
        status |= QTest::qExec(&tc, argc, argv);
    }

    {
        TestAgentConfigState tc;
        status |= QTest::qExec(&tc, argc, argv);
    }

    {
        TestAgentConfigTab tc;
        status |= QTest::qExec(&tc, argc, argv);
    }

    return status;
}

#include "tst_widgets.moc"
