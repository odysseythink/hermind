#include "icon_button.h"
#include "theme_colors.h"
#include "theme_style_helper.h"

IconButton::IconButton(QWidget *parent)
    : QToolButton(parent)
{
    setFixedSize(28, 28);
    setCursor(Qt::PointingHandCursor);
    setAutoRaise(true);

    new ThemeStyleHelper(this, [](QWidget *w, bool dark) {
        auto *btn = qobject_cast<IconButton *>(w);
        if (btn)
            btn->applyStyle(dark);
    }, this);
}

void IconButton::setIconText(const QString &text)
{
    setText(text);
}

void IconButton::applyStyle(bool dark)
{
    const QString bg = ThemeColors::hoverBackground(dark).name();
    const QString fg = ThemeColors::textSecondary(dark).name();
    const QString primary = ThemeColors::primary(dark).name();
    const QString primaryHover = ThemeColors::primaryHover(dark).name();

    const bool isSend = objectName() == QLatin1String("sendButton");
    const QString backColor = isSend ? primary : QStringLiteral("transparent");
    const QString backHover = isSend ? primaryHover : bg;
    const QString textColor = isSend ? QStringLiteral("#FFFFFF") : fg;

    setStyleSheet(QStringLiteral(
        "QToolButton {"
        "  border: none;"
        "  border-radius: 6px;"
        "  background-color: %1;"
        "  color: %2;"
        "  font-size: 14px;"
        "  padding: 0px;"
        "}"
        "QToolButton:hover {"
        "  background-color: %3;"
        "}"
    ).arg(backColor, textColor, backHover));
}
