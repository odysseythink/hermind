#include <QtTest>
#include <QSignalSpy>
#include <QLabel>
#include <QPushButton>
#include "default_chat_widget.h"
#include "quick_actions.h"
#include "hermind_api_client.h"

class TestDefaultChatWidget : public QObject
{
    Q_OBJECT
private slots:
    void setUsernameUpdatesGreeting();
    void emptyUsername_showsDefaultGreeting();
    void workspaceButtonEmitsSignal();
    void quickActions_hiddenUntilPhase2();
};

void TestDefaultChatWidget::setUsernameUpdatesGreeting()
{
    HermindApiClient client;
    DefaultChatWidget widget(&client);
    widget.setUsername(QStringLiteral("测试用户"));

    QList<QLabel *> labels = widget.findChildren<QLabel *>();
    bool found = false;
    for (auto *l : labels) {
        if (l->text().contains(QStringLiteral("测试用户"))) {
            found = true;
            break;
        }
    }
    QVERIFY(found);
}

void TestDefaultChatWidget::workspaceButtonEmitsSignal()
{
    HermindApiClient client;
    DefaultChatWidget widget(&client);
    widget.setWorkspaceSlug(QStringLiteral("my-ws"));

    QSignalSpy spy(&widget, &DefaultChatWidget::workspaceSelected);

    QPushButton *wsBtn = nullptr;
    QList<QPushButton *> btns = widget.findChildren<QPushButton *>();
    for (auto *b : btns) {
        if (b->text().contains(QStringLiteral("my-ws"))) {
            wsBtn = b;
            break;
        }
    }
    QVERIFY2(wsBtn != nullptr, "workspace button with slug text must exist");

    QTest::mouseClick(wsBtn, Qt::LeftButton);

    QCOMPARE(spy.count(), 1);
    QCOMPARE(spy.takeFirst().at(0).toString(), QStringLiteral("my-ws"));
}

// Single-user mode yields an empty username; the greeting must not render a
// dangling "欢迎回来, !".
void TestDefaultChatWidget::emptyUsername_showsDefaultGreeting()
{
    HermindApiClient client;
    DefaultChatWidget widget(&client);
    widget.setUsername(QStringLiteral("alice"));
    widget.setUsername(QString());

    QList<QLabel *> labels = widget.findChildren<QLabel *>();
    QString greeting;
    for (auto *l : labels) {
        if (l->text().contains(QStringLiteral("欢迎回来"))) {
            greeting = l->text();
            break;
        }
    }
    QVERIFY(!greeting.isEmpty());
    QVERIFY(!greeting.contains(QStringLiteral("alice")));
    QVERIFY(!greeting.contains(QStringLiteral(", !")));
}

// The quick-action buttons (create agent / edit workspace / upload document)
// target workspace-settings pages that only arrive in Phase 2, and the
// frontend reference has no equivalent — they must stay hidden for now
// rather than being clickable no-ops.
void TestDefaultChatWidget::quickActions_hiddenUntilPhase2()
{
    HermindApiClient client;
    DefaultChatWidget widget(&client);

    QuickActions *qa = widget.findChild<QuickActions *>();
    QVERIFY(qa != nullptr);
    QVERIFY(qa->isHidden());
}

QTEST_MAIN(TestDefaultChatWidget)
#include "tst_default_chat_widget.moc"
