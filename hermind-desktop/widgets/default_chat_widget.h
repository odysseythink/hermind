#ifndef DEFAULT_CHAT_WIDGET_H
#define DEFAULT_CHAT_WIDGET_H

#include <QWidget>
#include "prompt_input.h"

class QLabel;
class QPushButton;
class QuickActions;
class SuggestedMessages;
class HermindApiClient;

class DefaultChatWidget : public QWidget
{
    Q_OBJECT
public:
    explicit DefaultChatWidget(HermindApiClient *apiClient, QWidget *parent = nullptr);

    void setUsername(const QString &username);
    void setLogoPath(const QString &path);
    void setWorkspaceSlug(const QString &slug);
    void setSuggestedMessages(const QStringList &messages);

    PromptInput *promptInput() const;

signals:
    void workspaceSelected(const QString &slug);
    void createAgentClicked();
    void editWorkspaceClicked();
    void uploadDocumentClicked();
    void sendRequested(const QString &text);

private:
    void applyTheme();

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;

    QLabel *m_logoLabel = nullptr;
    QLabel *m_greetingLabel = nullptr;
    QPushButton *m_workspaceButton = nullptr;
    PromptInput *m_promptInput = nullptr;
    QuickActions *m_quickActions = nullptr;
    SuggestedMessages *m_suggestedMsgs = nullptr;
};

#endif // DEFAULT_CHAT_WIDGET_H
