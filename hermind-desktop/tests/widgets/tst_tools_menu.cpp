#include <QtTest>
#include <QSignalSpy>
#include <QListWidget>
#include "tools_menu.h"
#include "slash_commands_tab.h"

class TestToolsMenu : public QObject
{
    Q_OBJECT
private slots:
    void menuShowsAndHides();
    void clickingSlashCommandEmitsSignal();
    void escapeKeyHidesMenu();
};

void TestToolsMenu::menuShowsAndHides()
{
    QWidget anchor;
    anchor.setGeometry(0, 0, 100, 40);
    anchor.show();

    ToolsMenu menu;
    menu.showBelow(&anchor);
    QVERIFY(menu.isVisible());

    menu.hide();
    QVERIFY(!menu.isVisible());
}

void TestToolsMenu::clickingSlashCommandEmitsSignal()
{
    QWidget anchor;
    anchor.show();

    ToolsMenu menu;
    menu.showBelow(&anchor);
    QTest::qWait(50);

    // Find SlashCommandsTab and select an item
    SlashCommandsTab *tab = menu.findChild<SlashCommandsTab *>();
    QVERIFY(tab != nullptr);

    QSignalSpy cmdSpy(tab, &SlashCommandsTab::commandSelected);

    QListWidget *list = tab->findChild<QListWidget *>();
    QVERIFY(list != nullptr);
    QVERIFY(list->count() > 0);

    list->setCurrentRow(0);
    QListWidgetItem *item = list->item(0);
    QVERIFY(item != nullptr);

    // Simulate click via mouse event on the item's region
    QRect itemRect = list->visualItemRect(item);
    QTest::mouseClick(list->viewport(), Qt::LeftButton, Qt::NoModifier,
                      itemRect.center());

    QCOMPARE(cmdSpy.count(), 1);
}

void TestToolsMenu::escapeKeyHidesMenu()
{
    QWidget anchor;
    anchor.show();

    ToolsMenu menu;
    menu.showBelow(&anchor);
    QVERIFY(menu.isVisible());

    QSignalSpy closedSpy(&menu, &ToolsMenu::closed);
    QTest::keyClick(&menu, Qt::Key_Escape);

    QVERIFY(!menu.isVisible() || closedSpy.count() > 0);
}

QTEST_MAIN(TestToolsMenu)
#include "tst_tools_menu.moc"
