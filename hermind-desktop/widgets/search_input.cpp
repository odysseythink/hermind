#include "search_input.h"
#include "theme_colors.h"
#include "theme_style_helper.h"

SearchInput::SearchInput(QWidget *parent)
    : QLineEdit(parent)
{
    setMinimumHeight(32);
    setCursor(Qt::IBeamCursor);

    new ThemeStyleHelper(this, [](QWidget *w, bool dark) {
        auto *input = qobject_cast<SearchInput *>(w);
        if (input)
            input->applyStyle(dark);
    }, this);
}

void SearchInput::applyStyle(bool dark)
{
    const QString bg = ThemeColors::inputBackground(dark).name();
    const QString border = ThemeColors::border(dark).name();
    const QString text = ThemeColors::textPrimary(dark).name();
    const QString placeholder = ThemeColors::textDisabled(dark).name();

    setStyleSheet(QStringLiteral(
        "QLineEdit {"
        "  background-color: %1;"
        "  border: 1px solid %2;"
        "  border-radius: 8px;"
        "  padding: 6px 10px;"
        "  color: %3;"
        "  font-size: 13px;"
        "}"
        "QLineEdit::placeholder {"
        "  color: %4;"
        "}"
    ).arg(bg, border, text, placeholder));
}
