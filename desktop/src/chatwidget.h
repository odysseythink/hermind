#ifndef CHATWIDGET_H
#define CHATWIDGET_H

#include <QWidget>
#include <QTimer>

class QScrollArea;
class QVBoxLayout;
class PromptInput;
class MessageBubble;
class HermindClient;
class QNetworkReply;
class SSEParser;

class ChatWidget : public QWidget
{
    Q_OBJECT
public:
    explicit ChatWidget(QWidget *parent = nullptr);
    void setClient(HermindClient *client);

public slots:
    void sendMessage(const QString &text);
    void loadSession(const QString &sessionId);
    void startNewSession();

private slots:
    void onStreamReadyRead();
    void onStreamFinished();
    void onRenderTimer();

private:
    void addMessageBubble(MessageBubble *bubble);
    void startStream();

    HermindClient *m_client;
    QScrollArea *m_scrollArea;
    QWidget *m_messagesContainer;
    QVBoxLayout *m_messagesLayout;
    PromptInput *m_promptInput;
    MessageBubble *m_currentBubble;
    SSEParser *m_sseParser;
    QNetworkReply *m_streamReply;
    QTimer *m_renderTimer;
    QString m_pendingMarkdown;
    int m_renderGeneration;
    bool m_isStreaming;
};

#endif
