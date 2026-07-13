#include "memory_card.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>

MemoryCard::MemoryCard(const HermindMemory &memory, QWidget *parent)
    : QWidget(parent)
    , m_memory(memory)
{
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(10, 8, 10, 8);

    m_contentLabel = new QLabel(memory.content(), this);
    m_contentLabel->setWordWrap(true);
    m_contentLabel->setStyleSheet(QStringLiteral("font-size: 12px;"));

    QHBoxLayout *btnLayout = new QHBoxLayout();
    QPushButton *editBtn = new QPushButton(QStringLiteral("编辑"), this);
    QPushButton *deleteBtn = new QPushButton(QStringLiteral("删除"), this);
    editBtn->setFlat(true);
    deleteBtn->setFlat(true);
    editBtn->setCursor(Qt::PointingHandCursor);
    deleteBtn->setCursor(Qt::PointingHandCursor);

    btnLayout->addStretch();
    btnLayout->addWidget(editBtn);
    btnLayout->addWidget(deleteBtn);
    layout->addWidget(m_contentLabel);
    layout->addLayout(btnLayout);

    connect(editBtn, &QPushButton::clicked, this, [this]() { emit editRequested(m_memory); });
    connect(deleteBtn, &QPushButton::clicked, this, [this]() { emit deleteRequested(m_memory.id()); });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void MemoryCard::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "MemoryCard { background-color: %1; border-radius: 10px; border: 1px solid %2; }"
    ).arg(dark ? QStringLiteral("#27272a") : QStringLiteral("#f8fafc"),
          dark ? QStringLiteral("#3f3f46") : QStringLiteral("#e2e8f0")));
}
