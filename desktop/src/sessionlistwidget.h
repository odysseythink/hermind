#ifndef SESSIONLISTWIDGET_H
#define SESSIONLISTWIDGET_H

#include <QWidget>

class QListWidget;
class QPushButton;

class SessionListWidget : public QWidget
{
    Q_OBJECT
public:
    explicit SessionListWidget(QWidget *parent = nullptr);

signals:
    void sessionSelected(QString sessionId);
    void newSessionRequested();

public slots:
    void addSession(const QString &id, const QString &title);
    void clearSessions();

private slots:
    void onItemClicked();
    void onNewChatClicked();

private:
    QListWidget *m_listWidget;
    QPushButton *m_newChatButton;
};

#endif
