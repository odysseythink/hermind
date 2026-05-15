#include "chatwidget.h"
#include "promptinput.h"
#include "messagebubble.h"
#include "conversationheader.h"
#include "emptystatewidget.h"
#include "httplib.h"
#include "sseparser.h"
#include "markdownrenderer.h"

#include <QScrollArea>
#include <QVBoxLayout>
#include <QStackedWidget>
#include <QNetworkReply>
#include <QJsonDocument>
#include <QJsonObject>
#include <QDebug>
#include <QDragEnterEvent>
#include <QDropEvent>
#include <QMimeData>
#include <QUrl>
#include <QFile>
#include <QFileInfo>

ChatWidget::ChatWidget(QWidget *parent)
    : QWidget(parent),
      m_client(nullptr),
      m_header(new ConversationHeader(this)),
      m_stack(new QStackedWidget(this)),
      m_messagesPage(new QWidget(this)),
      m_scrollArea(new QScrollArea(m_messagesPage)),
      m_messagesContainer(new QWidget(m_messagesPage)),
      m_messagesLayout(new QVBoxLayout(m_messagesContainer)),
      m_emptyState(new EmptyStateWidget(this)),
      m_promptInput(new PromptInput(this)),
      m_currentBubble(nullptr),
      m_sseParser(new SSEParser(this)),
      m_streamReply(nullptr),
      m_renderTimer(new QTimer(this)),
      m_renderGeneration(0),
      m_isStreaming(false)
{
    // Message list page
    m_messagesLayout->setContentsMargins(16, 16, 16, 16);
    m_messagesLayout->setSpacing(16);
    m_messagesLayout->addStretch(1);

    m_scrollArea->setWidget(m_messagesContainer);
    m_scrollArea->setWidgetResizable(true);
    m_scrollArea->setFrameStyle(QFrame::NoFrame);

    QVBoxLayout *msgPageLayout = new QVBoxLayout(m_messagesPage);
    msgPageLayout->setContentsMargins(0, 0, 0, 0);
    msgPageLayout->setSpacing(0);
    msgPageLayout->addWidget(m_scrollArea, 1);

    m_stack->addWidget(m_emptyState);
    m_stack->addWidget(m_messagesPage);
    m_stack->setCurrentIndex(0);

    // Main layout
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(0);
    layout->addWidget(m_header);
    layout->addWidget(m_stack, 1);
    layout->addWidget(m_promptInput);

    connect(m_promptInput, &PromptInput::sendClicked,
            this, [this]() { sendMessage(m_promptInput->text()); });

    connect(m_emptyState, &EmptyStateWidget::suggestionClicked,
            this, &ChatWidget::sendMessage);

    connect(m_sseParser, &SSEParser::eventReceived,
            this, [this](const QString &, const QString &data) {
        QJsonDocument doc = QJsonDocument::fromJson(data.toUtf8());
        QJsonObject obj = doc.object();
        QString type = obj.value("type").toString();

        if (type == "message_chunk") {
            QJsonObject payload = obj.value("data").toObject();
            QString text = payload.value("text").toString();
            if (m_currentBubble) {
                m_currentBubble->appendMarkdown(text);
                m_pendingMarkdown = m_currentBubble->markdownBuffer();
                m_renderTimer->start(150);
            }
        } else if (type == "tool_call") {
            QJsonObject payload = obj.value("data").toObject();
            QString id = payload.value("id").toString();
            QString name = payload.value("name").toString();
            QString status = payload.value("status").toString();
            if (id.isEmpty()) id = name;
            if (m_currentBubble) {
                if (status == "started") {
                    m_currentBubble->addToolCall(id, name, status);
                } else {
                    m_currentBubble->updateToolCall(id, status);
                }
            }
        } else if (type == "done") {
            m_isStreaming = false;
            m_renderTimer->stop();
            if (!m_pendingMarkdown.isEmpty()) {
                onRenderTimer();
            }
            if (m_streamReply) {
                m_streamReply->deleteLater();
                m_streamReply = nullptr;
            }
        } else if (type == "error") {
            m_isStreaming = false;
            m_renderTimer->stop();
            if (m_currentBubble) {
                m_currentBubble->appendMarkdown("\n\n*[Error]*");
                onRenderTimer();
            }
            if (m_streamReply) {
                m_streamReply->deleteLater();
                m_streamReply = nullptr;
            }
        }
    });

    connect(m_renderTimer, &QTimer::timeout,
            this, &ChatWidget::onRenderTimer);

    m_renderTimer->setSingleShot(true);

    setAcceptDrops(true);
}

void ChatWidget::setClient(HermindClient *client)
{
    m_client = client;
    m_emptyState->setClient(client);
}

HermindClient* ChatWidget::client() const
{
    return m_client;
}

void ChatWidget::sendMessage(const QString &text)
{
    if (!m_client || text.isEmpty())
        return;

    setEmptyState(false);

    MessageBubble *userBubble = new MessageBubble(true, this);
    userBubble->setHtmlContent(text.toHtmlEscaped());
    addMessageBubble(userBubble);

    m_currentBubble = new MessageBubble(false, this);
    addMessageBubble(m_currentBubble);
    m_pendingMarkdown.clear();
    m_renderGeneration = 0;

    QJsonObject body;
    body["user_message"] = text;
    m_client->post("/api/conversation/messages", body,
                   [this](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) {
            qWarning() << "Failed to send message:" << error;
            if (m_currentBubble) {
                m_currentBubble->setHtmlContent("<i>Failed to send message</i>");
            }
            return;
        }
        startStream();
    });
}

void ChatWidget::startStream()
{
    m_isStreaming = true;
    m_streamReply = m_client->getStream("/api/sse");
    connect(m_streamReply, &QNetworkReply::readyRead,
            this, &ChatWidget::onStreamReadyRead);
    connect(m_streamReply, &QNetworkReply::finished,
            this, &ChatWidget::onStreamFinished);
}

void ChatWidget::onStreamReadyRead()
{
    if (m_streamReply) {
        m_sseParser->feed(m_streamReply->readAll());
    }
}

void ChatWidget::onStreamFinished()
{
    m_isStreaming = false;
    m_renderTimer->stop();
    if (!m_pendingMarkdown.isEmpty()) {
        onRenderTimer();
    }
    if (m_streamReply) {
        m_streamReply->deleteLater();
        m_streamReply = nullptr;
    }
}

void ChatWidget::onRenderTimer()
{
    if (m_pendingMarkdown.isEmpty())
        return;

    QString markdown = m_pendingMarkdown;
    m_pendingMarkdown.clear();

    if (m_currentBubble) {
        QString html = MarkdownRenderer::render(markdown, m_client);
        m_currentBubble->setHtmlContent(html);
    }
}

void ChatWidget::addMessageBubble(MessageBubble *bubble)
{
    m_messagesLayout->insertWidget(m_messagesLayout->count() - 1, bubble);

    if (!bubble->isUser()) {
        connect(bubble, &MessageBubble::deleteClicked, this, [this, bubble]() {
            deleteMessageBubble(bubble);
        });
        connect(bubble, &MessageBubble::regenerateClicked, this, [this, bubble]() {
            regenerateMessageBubble(bubble);
        });
    }
}

void ChatWidget::setEmptyState(bool empty)
{
    m_stack->setCurrentIndex(empty ? 0 : 1);
}

void ChatWidget::appendToCurrentBubble(const QString &text)
{
    if (m_currentBubble) {
        m_currentBubble->appendMarkdown(text);
    }
}

void ChatWidget::finalizeCurrentBubble()
{
    m_isStreaming = false;
    m_renderTimer->stop();
    if (!m_pendingMarkdown.isEmpty()) {
        onRenderTimer();
    }
}

void ChatWidget::loadSession(const QString &sessionId)
{
    Q_UNUSED(sessionId)
}

void ChatWidget::dragEnterEvent(QDragEnterEvent *event)
{
    if (event->mimeData()->hasUrls()) {
        event->acceptProposedAction();
    }
}

void ChatWidget::dropEvent(QDropEvent *event)
{
    const QMimeData *mime = event->mimeData();
    if (!mime->hasUrls() || !m_client)
        return;

    for (const QUrl &url : mime->urls()) {
        QString path = url.toLocalFile();
        if (path.isEmpty())
            continue;
        QFile file(path);
        if (!file.open(QIODevice::ReadOnly))
            continue;
        QByteArray data = file.readAll();
        QString fileName = QFileInfo(path).fileName();

        m_client->upload("/api/upload", data, fileName, QStringLiteral("application/octet-stream"),
            [this, fileName](const QJsonObject &resp, const QString &error) {
                if (!error.isEmpty()) {
                    qWarning() << "Upload failed:" << error;
                    return;
                }
                QString fileUrl = resp.value(QStringLiteral("url")).toString();
                m_promptInput->insertText(QStringLiteral("[%1](%2) ").arg(fileName, fileUrl));
            });
    }
}

void ChatWidget::cancelGeneration()
{
    if (m_streamReply) {
        m_streamReply->abort();
        m_streamReply->deleteLater();
        m_streamReply = nullptr;
    }
    m_isStreaming = false;
    m_renderTimer->stop();
    if (m_currentBubble) {
        m_currentBubble->setOperationsEnabled(true);
    }
}

void ChatWidget::deleteMessageBubble(MessageBubble *bubble)
{
    if (bubble == m_currentBubble) {
        cancelGeneration();
        m_currentBubble = nullptr;
    }
    m_messagesLayout->removeWidget(bubble);
    bubble->deleteLater();

    if (!bubble->messageId().isEmpty() && m_client) {
        m_client->delete_(QStringLiteral("/api/conversation/messages/%1").arg(bubble->messageId()),
                          [](const QJsonObject &, const QString &){});
    }
}

void ChatWidget::regenerateMessageBubble(MessageBubble *bubble)
{
    int index = m_messagesLayout->indexOf(bubble);
    if (index <= 0)
        return;

    QLayoutItem *item = m_messagesLayout->itemAt(index - 1);
    MessageBubble *userBubble = qobject_cast<MessageBubble*>(item->widget());
    if (!userBubble || !userBubble->isUser())
        return;

    deleteMessageBubble(bubble);

    m_currentBubble = new MessageBubble(false, this);
    addMessageBubble(m_currentBubble);
    m_pendingMarkdown.clear();
    m_renderGeneration = 0;

    QJsonObject body;
    body[QStringLiteral("user_message")] = userBubble->markdownBuffer();
    m_client->post(QStringLiteral("/api/conversation/messages"), body,
                   [this](const QJsonObject &resp, const QString &error) {
        Q_UNUSED(resp)
        if (!error.isEmpty()) {
            qWarning() << "Failed to regenerate:" << error;
            if (m_currentBubble) {
                m_currentBubble->setHtmlContent(QStringLiteral("<i>Failed to regenerate</i>"));
            }
            return;
        }
        startStream();
    });
}

void ChatWidget::startNewSession()
{
    while (m_messagesLayout->count() > 1) {
        QLayoutItem *item = m_messagesLayout->takeAt(0);
        if (item->widget()) {
            item->widget()->deleteLater();
        }
        delete item;
    }
    m_currentBubble = nullptr;
    m_pendingMarkdown.clear();
    m_renderGeneration = 0;
    m_isStreaming = false;
    if (m_streamReply) {
        m_streamReply->abort();
        m_streamReply->deleteLater();
        m_streamReply = nullptr;
    }
    setEmptyState(true);
    m_header->setTitle(QStringLiteral("New Conversation"));
}
