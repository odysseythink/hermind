#ifndef TOOLCALLWIDGET_H
#define TOOLCALLWIDGET_H

#include <QWidget>
#include <QString>

class QLabel;

class ToolCallWidget : public QWidget
{
    Q_OBJECT
public:
    explicit ToolCallWidget(const QString &name, const QString &status, QWidget *parent = nullptr);
    void setStatus(const QString &status);
    QString name() const;

private:
    void setupUI();
    void updateAppearance();

    QString m_name;
    QString m_status;
    QLabel *m_iconLabel;
    QLabel *m_nameLabel;
    QLabel *m_statusLabel;
};

#endif
