#ifndef AGENT_STATUS_BANNER_H
#define AGENT_STATUS_BANNER_H

#include <QWidget>

class QLabel;

class AgentStatusBanner : public QWidget
{
    Q_OBJECT
public:
    explicit AgentStatusBanner(QWidget *parent = nullptr);

public slots:
    void showStatus(const QString &status);
    void showClarification(const QString &question);
    void hideBanner();

private:
    void applyTheme();

    QLabel *m_label = nullptr;
};

#endif // AGENT_STATUS_BANNER_H
