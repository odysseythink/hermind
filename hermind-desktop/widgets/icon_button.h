#ifndef ICON_BUTTON_H
#define ICON_BUTTON_H

#include <QToolButton>

class IconButton : public QToolButton
{
    Q_OBJECT

public:
    explicit IconButton(QWidget *parent = nullptr);

    void setIconText(const QString &text);

private:
    void applyStyle(bool dark);
};

#endif // ICON_BUTTON_H
