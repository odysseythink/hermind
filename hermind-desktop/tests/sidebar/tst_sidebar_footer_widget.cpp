#include <QtTest>
#include <QSignalSpy>
#include "sidebar_footer_widget.h"

class TestSidebarFooterWidget : public QObject
{
    Q_OBJECT
private slots:
    void githubButtonEmitsOpenGitHub();
    void settingsButtonEmitsOpenSettings();
};

void TestSidebarFooterWidget::githubButtonEmitsOpenGitHub()
{
    SidebarFooterWidget footer;
    QSignalSpy spy(&footer, &SidebarFooterWidget::openGitHubRequested);
    footer.clickGitHubButton();
    QCOMPARE(spy.count(), 1);
}

void TestSidebarFooterWidget::settingsButtonEmitsOpenSettings()
{
    SidebarFooterWidget footer;
    QSignalSpy spy(&footer, &SidebarFooterWidget::openSettingsRequested);
    footer.clickSettingsButton();
    QCOMPARE(spy.count(), 1);
}

QTEST_MAIN(TestSidebarFooterWidget)
#include "tst_sidebar_footer_widget.moc"
