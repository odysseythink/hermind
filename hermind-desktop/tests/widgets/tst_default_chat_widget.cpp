#include <QtTest>
#include <QSignalSpy>
#include <QLabel>
#include <QPushButton>
#include "default_chat_widget.h"
#include "hermind_api_client.h"

class TestDefaultChatWidget : public QObject
{
    Q_OBJECT
private slots:
    void setUsernameUpdatesGreeting();
    void workspaceButtonEmitsSignal();
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

QTEST_MAIN(TestDefaultChatWidget)
#include "tst_default_chat_widget.moc"
