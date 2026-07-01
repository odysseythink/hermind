#ifndef SETTING_ROW_H
#define SETTING_ROW_H

#include <QWidget>
#include <QLabel>

class SettingRow : public QWidget
{
    Q_OBJECT

public:
    explicit SettingRow(QWidget *parent = nullptr);

    void setTitle(const QString &title);
    void setDescription(const QString &description);
    void setControl(QWidget *control);

private:
    void applyStyle(bool dark);
    void rebuildLayout();

    QLabel *m_titleLabel = nullptr;
    QLabel *m_descLabel = nullptr;
    QWidget *m_control = nullptr;
};

#endif // SETTING_ROW_H
