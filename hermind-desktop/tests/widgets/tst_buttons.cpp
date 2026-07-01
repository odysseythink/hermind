#include <QtTest>
#include <QApplication>
#include "icon_button.h"
#include "sidebar_menu_button.h"

class TestButtons : public QObject
{
    Q_OBJECT

private slots:
    void iconButtonHasFixedSize();
    void sidebarMenuButtonIsCheckable();
    void sidebarMenuButtonIsChecked();
};

void TestButtons::iconButtonHasFixedSize()
{
    IconButton btn;
    QCOMPARE(btn.minimumSize(), QSize(28, 28));
    QCOMPARE(btn.maximumSize(), QSize(28, 28));
}

void TestButtons::sidebarMenuButtonIsCheckable()
{
    SidebarMenuButton btn(QStringLiteral("Appearance"));
    QVERIFY(btn.isCheckable());
    QVERIFY(btn.isFlat());
}

void TestButtons::sidebarMenuButtonIsChecked()
{
    SidebarMenuButton btn(QStringLiteral("Appearance"));
    btn.setChecked(true);
    QVERIFY(btn.isChecked());
}

QTEST_MAIN(TestButtons)
#include "tst_buttons.moc"
