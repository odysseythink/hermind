#include "ShortcutManager.h"
#include <QAbstractEventDispatcher>
#include <QDebug>

#ifdef Q_OS_WIN
#include <windows.h>

static int qtKeyToVk(int key)
{
    key &= ~Qt::KeyboardModifierMask;
    if (key >= Qt::Key_A && key <= Qt::Key_Z)
        return key - Qt::Key_A + 'A';
    if (key >= Qt::Key_0 && key <= Qt::Key_9)
        return key - Qt::Key_0 + '0';
    if (key >= Qt::Key_F1 && key <= Qt::Key_F24)
        return VK_F1 + (key - Qt::Key_F1);
    switch (key) {
    case Qt::Key_Space: return VK_SPACE;
    case Qt::Key_Escape: return VK_ESCAPE;
    case Qt::Key_Tab: return VK_TAB;
    case Qt::Key_Backspace: return VK_BACK;
    case Qt::Key_Return: return VK_RETURN;
    case Qt::Key_Enter: return VK_RETURN;
    case Qt::Key_Insert: return VK_INSERT;
    case Qt::Key_Delete: return VK_DELETE;
    case Qt::Key_Pause: return VK_PAUSE;
    case Qt::Key_Print: return VK_PRINT;
    case Qt::Key_Home: return VK_HOME;
    case Qt::Key_End: return VK_END;
    case Qt::Key_Left: return VK_LEFT;
    case Qt::Key_Up: return VK_UP;
    case Qt::Key_Right: return VK_RIGHT;
    case Qt::Key_Down: return VK_DOWN;
    case Qt::Key_PageUp: return VK_PRIOR;
    case Qt::Key_PageDown: return VK_NEXT;
    case Qt::Key_Shift: return VK_SHIFT;
    case Qt::Key_Control: return VK_CONTROL;
    case Qt::Key_Alt: return VK_MENU;
    case Qt::Key_CapsLock: return VK_CAPITAL;
    case Qt::Key_NumLock: return VK_NUMLOCK;
    case Qt::Key_ScrollLock: return VK_SCROLL;
    case Qt::Key_Comma: return VK_OEM_COMMA;
    case Qt::Key_Period: return VK_OEM_PERIOD;
    case Qt::Key_Slash: return VK_OEM_2;
    case Qt::Key_Semicolon: return VK_OEM_1;
    case Qt::Key_QuoteLeft: return VK_OEM_3;
    case Qt::Key_BracketLeft: return VK_OEM_4;
    case Qt::Key_Backslash: return VK_OEM_5;
    case Qt::Key_BracketRight: return VK_OEM_6;
    case Qt::Key_QuoteDbl: return VK_OEM_7;
    case Qt::Key_Minus: return VK_OEM_MINUS;
    case Qt::Key_Plus: return VK_OEM_PLUS;
    case Qt::Key_Equal: return VK_OEM_PLUS;
    default: return 0;
    }
}

int ShortcutManager::s_nextHotkeyId = 1;
#endif

ShortcutManager::ShortcutManager(QObject *parent)
    : QObject(parent),
      m_registered(false)
#ifdef Q_OS_WIN
    , m_hotkeyId(s_nextHotkeyId++)
#endif
{
}

ShortcutManager::~ShortcutManager()
{
    unregisterAll();
}

bool ShortcutManager::registerToggle(const QKeySequence &seq)
{
#ifdef Q_OS_WIN
    if (m_registered)
        return true;

    int keyCombo = seq[0].key();
    UINT modifiers = 0;
    if (keyCombo & Qt::CTRL) modifiers |= MOD_CONTROL;
    if (keyCombo & Qt::ALT) modifiers |= MOD_ALT;
    if (keyCombo & Qt::SHIFT) modifiers |= MOD_SHIFT;
    if (keyCombo & Qt::META) modifiers |= MOD_WIN;

    UINT vk = qtKeyToVk(keyCombo);
    if (vk == 0) {
        qWarning() << "Unsupported key in shortcut:" << seq.toString();
        return false;
    }

    if (RegisterHotKey(NULL, m_hotkeyId, modifiers, vk)) {
        QAbstractEventDispatcher *ed = QAbstractEventDispatcher::instance();
        if (ed)
            ed->installNativeEventFilter(this);
        m_registered = true;
        return true;
    }
    qWarning() << "Failed to register global hotkey:" << seq.toString();
    return false;
#else
    Q_UNUSED(seq)
    qWarning() << "Global shortcuts not implemented on this platform";
    return false;
#endif
}

void ShortcutManager::unregisterAll()
{
#ifdef Q_OS_WIN
    if (m_registered) {
        UnregisterHotKey(NULL, m_hotkeyId);
        QAbstractEventDispatcher *ed = QAbstractEventDispatcher::instance();
        if (ed)
            ed->removeNativeEventFilter(this);
        m_registered = false;
    }
#else
    Q_UNUSED(m_registered)
#endif
}

bool ShortcutManager::nativeEventFilter(const QByteArray &eventType, void *message, qintptr *result)
{
#ifdef Q_OS_WIN
    if (eventType == "windows_generic_MSG" || eventType == "windows_dispatcher_MSG") {
        MSG *msg = static_cast<MSG *>(message);
        if (msg->message == WM_HOTKEY && msg->wParam == static_cast<WPARAM>(m_hotkeyId)) {
            emit toggleRequested();
            if (result)
                *result = 0;
            return true;
        }
    }
#else
    Q_UNUSED(eventType)
    Q_UNUSED(message)
    Q_UNUSED(result)
#endif
    return false;
}
