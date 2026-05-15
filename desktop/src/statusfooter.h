#ifndef STATUSFOOTER_H
#define STATUSFOOTER_H

#include <QWidget>

class QLabel;

class StatusFooter : public QWidget
{
    Q_OBJECT
public:
    explicit StatusFooter(QWidget *parent = nullptr);

    void setVersion(const QString &version);
    void setModel(const QString &model);
    void setStatus(const QString &status);

private:
    void setupUI();
    QLabel *m_label;
};

#endif
