#ifndef SHORTCUTMANAGER_H
#define SHORTCUTMANAGER_H

#include <QObject>

class ShortcutManager : public QObject
{
    Q_OBJECT
public:
    explicit ShortcutManager(QObject *parent = nullptr);
    bool registerToggle(const QKeySequence &seq);

signals:
    void toggleRequested();

private:
    bool m_registered;
};

#endif
