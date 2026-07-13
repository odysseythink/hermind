#ifndef TOOLS_MENU_H
#define TOOLS_MENU_H

#include <QWidget>
#include <functional>

class SlashCommandsTab;

class ToolsMenu : public QWidget
{
    Q_OBJECT
public:
    explicit ToolsMenu(QWidget *parent = nullptr);

    void setSendCommandCallback(std::function<void(const QString &, const QString &)> callback);
    void showAbove(QWidget *anchor);
    void showBelow(QWidget *anchor);

signals:
    void closed();

protected:
    void focusOutEvent(QFocusEvent *event) override;
    void keyPressEvent(QKeyEvent *event) override;

private:
    void applyTheme();

    SlashCommandsTab *m_slashTab = nullptr;
    std::function<void(const QString &, const QString &)> m_callback;
};

#endif // TOOLS_MENU_H
