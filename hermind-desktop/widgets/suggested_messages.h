#ifndef SUGGESTED_MESSAGES_H
#define SUGGESTED_MESSAGES_H

#include <QWidget>
#include <QStringList>
#include <functional>

class QVBoxLayout;

class SuggestedMessages : public QWidget
{
    Q_OBJECT
public:
    explicit SuggestedMessages(QWidget *parent = nullptr);

    void setMessages(const QStringList &messages);
    void setSendCommandCallback(std::function<void(const QString &, const QString &)> callback);

private:
    void rebuild();

    QStringList m_messages;
    QVBoxLayout *m_layout = nullptr;
    std::function<void(const QString &, const QString &)> m_callback;
};

#endif // SUGGESTED_MESSAGES_H
