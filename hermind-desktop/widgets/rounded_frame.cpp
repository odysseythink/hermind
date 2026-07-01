#include "rounded_frame.h"
#include "theme_colors.h"
#include "theme_style_helper.h"
#include "theme_manager.h"

RoundedFrame::RoundedFrame(QWidget *parent)
    : QFrame(parent)
{
    setFrameShape(QFrame::StyledPanel);
    setFrameShadow(QFrame::Plain);

    new ThemeStyleHelper(this, [](QWidget *w, bool dark) {
        auto *frame = qobject_cast<RoundedFrame *>(w);
        if (frame)
            frame->applyStyle(dark);
    }, this);
}

void RoundedFrame::setRadius(int radius)
{
    m_radius = radius;
    applyStyle(ThemeManager::instance().isDarkMode());
}

void RoundedFrame::applyStyle(bool dark)
{
    const QString bg = ThemeColors::cardBackground(dark).name();
    const QString border = ThemeColors::border(dark).name();

    setStyleSheet(QStringLiteral(
        "RoundedFrame {"
        "  background-color: %1;"
        "  border: 1px solid %2;"
        "  border-radius: %3px;"
        "}"
    ).arg(bg, border).arg(m_radius));
}
