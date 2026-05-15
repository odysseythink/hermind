#ifndef MESSAGEBUBBLE_H
#define MESSAGEBUBBLE_H

#include <QWidget>
#include <QString>
#include <QHash>

class QTextEdit;
class QLabel;
class QHBoxLayout;
class QWidget;
class ToolCallWidget;

class MessageBubble : public QWidget
{
    Q_OBJECT
public:
    explicit MessageBubble(bool isUser, QWidget *parent = nullptr);

    void appendMarkdown(const QString &text);
    QString markdownBuffer() const;
    void setHtmlContent(const QString &html);

    bool isUser() const;
    void setMessageId(const QString &id);
    QString messageId() const;

    void setOperationsEnabled(bool enabled);
    void setEditable(bool editable);

    void addToolCall(const QString &id, const QString &name, const QString &status);
    void updateToolCall(const QString &id, const QString &status);

signals:
    void copyClicked();
    void editClicked();
    void deleteClicked();
    void regenerateClicked();

private slots:
    void onCopyClicked();
    void onEditClicked();
    void onDeleteClicked();
    void onRegenerateClicked();

private:
    void setupUI();
    void setupOperations();

    bool m_isUser;
    bool m_editable;
    QString m_messageId;
    QString m_markdownBuffer;
    QLabel *m_roleTag;
    QTextEdit *m_content;
    QWidget *m_operationsBar;
    QHBoxLayout *m_operationsLayout;
    QWidget *m_toolCallArea;
    QHash<QString, ToolCallWidget*> m_toolCalls;
};

#endif
