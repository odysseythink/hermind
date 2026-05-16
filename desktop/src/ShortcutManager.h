#ifndef SHORTCUTMANAGER_H
#define SHORTCUTMANAGER_H

#include <QObject>
#include <QKeySequence>
#include <QAbstractNativeEventFilter>

class ShortcutManager : public QObject, public QAbstractNativeEventFilter
{
    Q_OBJECT
public:
    explicit ShortcutManager(QObject *parent = nullptr);
    ~ShortcutManager();

    bool registerToggle(const QKeySequence &seq);
    void unregisterAll();

    bool nativeEventFilter(const QByteArray &eventType, void *message, qintptr *result) override;

signals:
    void toggleRequested();

private:
    bool m_registered;
#ifdef Q_OS_WIN
    int m_hotkeyId;
    static int s_nextHotkeyId;
#endif
};

#endif
