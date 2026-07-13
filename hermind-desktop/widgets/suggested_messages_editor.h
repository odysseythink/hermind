#ifndef SUGGESTED_MESSAGES_EDITOR_H
#define SUGGESTED_MESSAGES_EDITOR_H

#include <QWidget>
#include <QStringList>

class QLineEdit;
class QPushButton;

class SuggestedMessagesEditor : public QWidget
{
    Q_OBJECT

public:
    explicit SuggestedMessagesEditor(QWidget *parent = nullptr);

    QStringList messages() const;
    QStringList validMessages() const; // non-empty trimmed
    bool hasChanges() const;

public slots:
    void setMessages(const QStringList &messages);
    void markSaved();

signals:
    void messagesChanged();
    void saveRequested();

private slots:
    void addMessage();
    void removeRow();
    void onTextChanged();
    void onSaveClicked();

private:
    void rebuild();
    void updateChangeState();
    void updateAddButton();

    struct Row {
        QWidget *container = nullptr;
        QLineEdit *edit = nullptr;
        QString originalText;
    };

    QList<Row> m_rows;
    QStringList m_initialMessages;
    QPushButton *m_addButton = nullptr;
    QPushButton *m_saveButton = nullptr;
    bool m_hasChanges = false;
    static constexpr int kMaxMessages = 4;
};

#endif // SUGGESTED_MESSAGES_EDITOR_H
