#include <QtTest>
#include "theme_colors.h"

class TestThemeColors : public QObject
{
    Q_OBJECT

private slots:
    void lightAndDarkDiffer();
    void primaryIsIndependentOfTheme();
    void colorsAreValid();
};

void TestThemeColors::lightAndDarkDiffer()
{
    QVERIFY(ThemeColors::windowBackground(false) != ThemeColors::windowBackground(true));
    QVERIFY(ThemeColors::sidebarBackground(false) != ThemeColors::sidebarBackground(true));
    QVERIFY(ThemeColors::textPrimary(false) != ThemeColors::textPrimary(true));
}

void TestThemeColors::primaryIsIndependentOfTheme()
{
    QCOMPARE(ThemeColors::primary(false), ThemeColors::primary(true));
}

void TestThemeColors::colorsAreValid()
{
    QVERIFY(ThemeColors::windowBackground(false).isValid());
    QVERIFY(ThemeColors::windowBackground(true).isValid());
    QVERIFY(ThemeColors::selectedBackground(true).isValid());
    QVERIFY(ThemeColors::separator(false).isValid());
}

QTEST_MAIN(TestThemeColors)
#include "tst_theme_colors.moc"
