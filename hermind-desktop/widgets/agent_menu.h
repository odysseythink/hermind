#ifndef AGENT_MENU_H
#define AGENT_MENU_H

#include <QWidget>
#include <functional>

class QPushButton;

class AgentMenu : public QWidget
{
    Q_OBJECT
public:
    explicit AgentMenu(QWidget *parent = nullptr);

    void showAt(const QPoint &globalPos);
    void setSendCommandCallback(std::function<void(const QString &, const QString &)> callback);

signals:
    void agentSelected();

protected:
    void focusOutEvent(QFocusEvent *event) override;

private:
    void applyTheme();

    QPushButton *m_agentItem = nullptr;
    std::function<void(const QString &, const QString &)> m_callback;
};

#endif // AGENT_MENU_H
