#ifndef PROMPT_INPUT_H
#define PROMPT_INPUT_H

#include <QWidget>
#include <QStringList>
#include <QMetaType>

class QTextEdit;
class QPushButton;
class AgentMenu;
class ToolsMenu;
class AttachItem;
class AttachmentManager;
class HermindApiClient;

// writeMode values for PromptCommand (see below). Kept as constants so the
// contract between producers (menus, suggested messages) and the consumer
// (ChatContainerWidget::sendCommand) cannot drift.
namespace WriteMode {
inline const QString Replace = QStringLiteral("replace");
inline const QString Prepend = QStringLiteral("prepend");
inline const QString Append = QStringLiteral("append");
}

struct PromptCommand {
    QString text;
    QString writeMode; // one of WriteMode::{Replace,Append,Prepend}
    QStringList attachments;
};

Q_DECLARE_METATYPE(PromptCommand)

class PromptInput : public QWidget
{
    Q_OBJECT
public:
    explicit PromptInput(QWidget *parent = nullptr);

    QString text() const;
    void setText(const QString &text);
    void clear();
    void setPlaceholderText(const QString &text);
    void setMaxHeight(int px);
    int maxHeight() const;

    void setSendEnabled(bool enabled);
    bool isSendEnabled() const;
    void setStopVisible(bool visible);

    void setApiClient(HermindApiClient *client);
    void setWorkspaceSlug(const QString &slug);
    bool isProcessingAttachments() const;

    QTextEdit *textEdit() const; // for external focus/font control
    QStringList attachments() const; // currently attached file paths

signals:
    void sendCommand(const PromptCommand &command);
    void stopRequested();

protected:
    bool eventFilter(QObject *obj, QEvent *event) override;

private slots:
    void onTextChanged();
    void adjustHeight();
    void sendCurrent();

private:
    void applyTheme();
    void updateSendEnabled();

    QTextEdit *m_textEdit = nullptr;
    QPushButton *m_agentButton = nullptr;
    QPushButton *m_toolsButton = nullptr;
    QPushButton *m_sendButton = nullptr;
    QPushButton *m_stopButton = nullptr;
    AgentMenu *m_agentMenu = nullptr;
    ToolsMenu *m_toolsMenu = nullptr;
    AttachItem *m_attachItem = nullptr;
    AttachmentManager *m_attachManager = nullptr;
    bool m_sendEnabled = true;

    int m_maxHeight = 200;
};

#endif // PROMPT_INPUT_H
