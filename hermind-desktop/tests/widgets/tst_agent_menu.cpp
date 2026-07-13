#include <QtTest>
#include <QSignalSpy>
#include <QPushButton>
#include "agent_menu.h"

class TestAgentMenu : public QObject
{
    Q_OBJECT
private slots:
    void clickingAgentItemEmitsAgentSelected();
    void clickingAgentItemInvokesCallback();
    void constructorHasAgentItemWithText();
};

void TestAgentMenu::clickingAgentItemEmitsAgentSelected()
{
    AgentMenu menu;
    QSignalSpy spy(&menu, &AgentMenu::agentSelected);

    QList<QPushButton *> btns = menu.findChildren<QPushButton *>();
    QVERIFY(!btns.isEmpty());
    QTest::mouseClick(btns.first(), Qt::LeftButton);

    QCOMPARE(spy.count(), 1);
}

void TestAgentMenu::clickingAgentItemInvokesCallback()
{
    AgentMenu menu;
    QString capturedText;
    QString capturedMode;

    menu.setSendCommandCallback([&](const QString &text, const QString &mode) {
        capturedText = text;
        capturedMode = mode;
    });

    QList<QPushButton *> btns = menu.findChildren<QPushButton *>();
    QVERIFY(!btns.isEmpty());
    QTest::mouseClick(btns.first(), Qt::LeftButton);

    QCOMPARE(capturedText, QStringLiteral("@agent "));
    QCOMPARE(capturedMode, QStringLiteral("prepend"));
}

void TestAgentMenu::constructorHasAgentItemWithText()
{
    AgentMenu menu;
    QList<QPushButton *> btns = menu.findChildren<QPushButton *>();
    QVERIFY(!btns.isEmpty());
    QVERIFY(btns.first()->text().contains(QStringLiteral("@agent")));
}

QTEST_MAIN(TestAgentMenu)
#include "tst_agent_menu.moc"
