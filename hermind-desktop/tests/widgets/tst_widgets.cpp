#include <QtTest>
#include <QApplication>
#include <QComboBox>
#include "icon_button.h"
#include "sidebar_menu_button.h"
#include "search_input.h"
#include "styled_separator.h"
#include "rounded_frame.h"
#include "setting_row.h"

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

QTEST_MAIN(TestWidgets)
#include "tst_widgets.moc"
