#include "suggested_messages.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QVBoxLayout>
#include <QPushButton>

SuggestedMessages::SuggestedMessages(QWidget *parent)
    : QWidget(parent)
{
    m_layout = new QVBoxLayout(this);
    m_layout->setSpacing(4);
    m_layout->addStretch();

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { rebuild(); });
}

void SuggestedMessages::setMessages(const QStringList &messages)
{
    m_messages = messages;
    rebuild();
}

void SuggestedMessages::setSendCommandCallback(std::function<void(const QString &, const QString &)> callback)
{
    m_callback = std::move(callback);
}

void SuggestedMessages::rebuild()
{
    // Remove old buttons (keep stretch)
    while (m_layout->count() > 1) {
        QLayoutItem *item = m_layout->takeAt(0);
        delete item->widget();
        delete item;
    }

    const bool dark = ThemeManager::instance().isDarkMode();
    for (const QString &msg : m_messages) {
        QPushButton *btn = new QPushButton(msg, this);
        btn->setFlat(true);
        btn->setCursor(Qt::PointingHandCursor);
        btn->setStyleSheet(QStringLiteral(
            "QPushButton { color: %1; font-size: 13px; padding: 6px 16px; border-radius: 8px; text-align: left; }"
            "QPushButton:hover { background-color: %2; }"
        ).arg(dark ? QStringLiteral("#a1a1aa") : QStringLiteral("#64748b"),
              dark ? QStringLiteral("#27272a") : QStringLiteral("#f1f5f9")));
        connect(btn, &QPushButton::clicked, this, [this, msg]() {
            if (m_callback)
                m_callback(msg, QStringLiteral("replace"));
        });
        m_layout->insertWidget(m_layout->count() - 1, btn);
    }
}
