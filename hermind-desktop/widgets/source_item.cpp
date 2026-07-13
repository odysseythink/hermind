#include "source_item.h"
#include "theme_colors.h"
#include "theme_manager.h"
#include <QVBoxLayout>
#include <QLabel>

SourceItem::SourceItem(const QJsonObject &source, QWidget *parent)
    : QWidget(parent)
    , m_source(source)
{
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 8, 8, 8);
    layout->setSpacing(2);

    QString title = source.value(QStringLiteral("title")).toString(
        source.value(QStringLiteral("name")).toString(QStringLiteral("Untitled")));
    QString description = source.value(QStringLiteral("description")).toString(
        source.value(QStringLiteral("text")).toString());

    // Truncate long descriptions
    if (description.length() > 120)
        description = description.left(120) + QStringLiteral("...");

    m_titleLabel = new QLabel(title, this);
    m_titleLabel->setStyleSheet(QStringLiteral("font-weight: 600; font-size: 12px;"));
    m_descLabel = new QLabel(description, this);
    m_descLabel->setWordWrap(true);
    m_descLabel->setStyleSheet(QStringLiteral("font-size: 11px; color: palette(text);"));

    layout->addWidget(m_titleLabel);
    layout->addWidget(m_descLabel);

    setCursor(Qt::PointingHandCursor);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void SourceItem::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "SourceItem { background-color: %1; border-radius: 8px; }"
        "SourceItem:hover { background-color: %2; }"
    ).arg(ThemeColors::cardBackground(dark).name(),
          ThemeColors::hoverBackground(dark).name()));
}

void SourceItem::mousePressEvent(QMouseEvent *)
{
    emit clicked(m_source);
}
