#ifndef SLASH_COMMANDS_TAB_H
#define SLASH_COMMANDS_TAB_H

#include <QWidget>
#include <QVector>
#include <functional>

class QListWidget;

struct SlashCommand {
    QString label;     // e.g. "/reset — 重置聊天"
    QString text;      // e.g. "/reset"
    QString writeMode; // e.g. "replace"
};

class SlashCommandsTab : public QWidget
{
    Q_OBJECT
public:
    explicit SlashCommandsTab(QWidget *parent = nullptr);

    void setSendCommandCallback(std::function<void(const QString &, const QString &)> callback);
    void handleArrowKey(int key);
    void activateHighlighted();

signals:
    void commandSelected(const QString &text, const QString &mode);

private:
    void applyTheme();

    QListWidget *m_list = nullptr;
    QVector<SlashCommand> m_commands;
    std::function<void(const QString &, const QString &)> m_callback;
};

#endif // SLASH_COMMANDS_TAB_H
