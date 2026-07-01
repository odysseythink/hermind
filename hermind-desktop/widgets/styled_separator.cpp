#include "styled_separator.h"
#include "theme_colors.h"
#include "theme_style_helper.h"

StyledSeparator::StyledSeparator(QWidget *parent)
    : QFrame(parent)
{
    setFrameShape(QFrame::HLine);
    setFrameShadow(QFrame::Plain);
    setMaximumHeight(1);
    setMinimumHeight(1);

    new ThemeStyleHelper(this, [](QWidget *w, bool dark) {
        auto *sep = qobject_cast<StyledSeparator *>(w);
        if (sep)
            sep->applyStyle(dark);
    }, this);
}

void StyledSeparator::applyStyle(bool dark)
{
    const QString color = ThemeColors::separator(dark).name();
    setStyleSheet(QStringLiteral(
        "QFrame {"
        "  color: %1;"
        "  background-color: %1;"
        "  border: none;"
        "  max-height: 1px;"
        "}"
    ).arg(color));
}
