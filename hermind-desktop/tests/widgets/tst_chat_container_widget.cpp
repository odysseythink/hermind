#include <QtTest>
#include <QSignalSpy>
#include "chat_container_widget.h"
#include "hermind_api_client.h"

class TestChatContainerWidget : public QObject
{
    Q_OBJECT
private slots:
    void setWorkspace_updatesWelcomeLabel();
    void sendButtonClicked_startsStream();
};

void TestChatContainerWidget::setWorkspace_updatesWelcomeLabel()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);

    widget.setWorkspace("ws-1", "My Workspace");

    QCOMPARE(widget.workspaceSlug(), QString("ws-1"));
    QCOMPARE(widget.workspaceName(), QString("My Workspace"));
}

void TestChatContainerWidget::sendButtonClicked_startsStream()
{
    HermindApiClient client;
    ChatContainerWidget widget(&client, nullptr);
    widget.setWorkspace("ws-1", "My Workspace");
    widget.setInputText("Hello");

    QSignalSpy spy(&widget, &ChatContainerWidget::streamStarted);
    widget.onSendClicked();

    QCOMPARE(spy.count(), 1);
}

QTEST_MAIN(TestChatContainerWidget)
#include "tst_chat_container_widget.moc"
