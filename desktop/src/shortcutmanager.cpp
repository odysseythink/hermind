#include "shortcutmanager.h"
#include <QShortcut>
#include <QWidget>

ShortcutManager::ShortcutManager(QObject *parent)
    : QObject(parent),
      m_registered(false)
{
}

bool ShortcutManager::registerToggle(const QKeySequence &seq)
{
    // TODO: Replace with true global shortcut using platform-native APIs
    // (macOS: CGEventTap/NSEvent global monitor, Windows: RegisterHotKey).
    // For now, this is an application-level shortcut that only works when
    // the app is focused.
    QWidget *window = qobject_cast<QWidget*>(parent());
    if (!window)
        return false;

    QShortcut *sc = new QShortcut(seq, window);
    connect(sc, &QShortcut::activated, this, &ShortcutManager::toggleRequested);
    m_registered = true;
    return true;
}
