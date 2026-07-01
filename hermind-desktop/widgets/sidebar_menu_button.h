#ifndef SIDEBAR_MENU_BUTTON_H
#define SIDEBAR_MENU_BUTTON_H

#include <QPushButton>

class SidebarMenuButton : public QPushButton
{
    Q_OBJECT

public:
    explicit SidebarMenuButton(const QString &text, QWidget *parent = nullptr);

private:
    void applyStyle(bool dark);
};

#endif // SIDEBAR_MENU_BUTTON_H
