#ifndef TOPBAR_H
#define TOPBAR_H

#include <QWidget>

class QLabel;
class QPushButton;

class TopBar : public QWidget
{
    Q_OBJECT
public:
    explicit TopBar(QWidget *parent = nullptr);

    void setStatus(const QString &status);
    void setStatusDotColor(const QString &color);

signals:
    void modeChanged(const QString &mode);
    void saveRequested();

private:
    void setupUI();

    QLabel *m_brandLabel;
    QPushButton *m_chatModeBtn;
    QPushButton *m_settingsModeBtn;
    QLabel *m_statusLabel;
    QWidget *m_statusDot;
    QPushButton *m_saveBtn;
};

#endif
