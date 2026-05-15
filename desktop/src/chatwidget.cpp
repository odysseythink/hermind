#include "chatwidget.h"
#include "promptinput.h"
#include "messagebubble.h"
#include "httplib.h"
#include "sseparser.h"

#include <QScrollArea>
#include <QVBoxLayout>
#include <QNetworkReply>
#include <QJsonDocument>
#include <QJsonObject>
#include <QDebug>
#include <QDragEnterEvent>
#include <QDropEvent>
#include <QMimeData>
#include <QUrl>
#include <QFile>

ChatWidget::ChatWidget(QWidget *parent)
    : QWidget(parent),
      m_client(nullptr),
      m_scrollArea(new QScrollArea(this)),
      m_messagesContainer(new QWidget(this)),
      m_messagesLayout(new QVBoxLayout(m_messagesContainer)),
      m_promptInput(new PromptInput(this)),
      m_currentBubble(nullptr),
      m_sseParser(new SSEParser(this)),
      m_streamReply(nullptr),
      m_renderTimer(new QTimer(this)),
      m_renderGeneration(0),
      m_isStreaming(false)
{
    m_messagesLayout->setContentsMargins(8, 8, 8, 8);
    m_messagesLayout->setSpacing(8);
    m_messagesLayout->addStretch(1);

    m_scrollArea->setWidget(m_messagesContainer);
    m_scrollArea->setWidgetResizable(true);
    m_scrollArea->setFrameStyle(QFrame::NoFrame);

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(0);
    layout->addWidget(m_scrollArea, 1);
    layout->addWidget(m_promptInput);

    connect(m_promptInput, &PromptInput::sendClicked,
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
}

HermindClient* ChatWidget::client() const
{
    return m_client;
}

void ChatWidget::sendMessage(const QString &text)
{
    if (!m_client)
        return;

    MessageBubble *userBubble = new MessageBubble("user", this);
    userBubble->setHtmlContent(text.toHtmlEscaped());
    addMessageBubble(userBubble);

    m_currentBubble = new MessageBubble("assistant", this);
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
    if (!m_client || m_pendingMarkdown.isEmpty())
        return;

    int gen = ++m_renderGeneration;
    QString markdown = m_pendingMarkdown;
    m_pendingMarkdown.clear();

    QJsonObject body;
    body["content"] = markdown;
    m_client->post("/api/render", body,
                   [this, gen](const QJsonObject &resp, const QString &error) {
        if (error.isEmpty() && m_currentBubble && gen == m_renderGeneration) {
            m_currentBubble->setHtmlContent(resp.value("html").toString());
        }
    });
}

void ChatWidget::addMessageBubble(MessageBubble *bubble)
{
    m_messagesLayout->insertWidget(m_messagesLayout->count() - 1, bubble);
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
    if (!mime->hasUrls())
        return;

    for (const QUrl &url : mime->urls()) {
        QString path = url.toLocalFile();
        if (path.isEmpty())
            continue;
        QFile file(path);
        if (!file.open(QIODevice::ReadOnly))
            continue;
        QByteArray data = file.readAll();
        qDebug() << "Dropped file:" << path << "size:" << data.size();
    }
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
}
