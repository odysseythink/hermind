#include "sources_sidebar.h"
#include "source_item.h"
#include "citation_detail_modal.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QScrollArea>
#include <QSet>
#include <QJsonObject>

SourcesSidebar::SourcesSidebar(QWidget *parent)
    : QWidget(parent)
{
    setFixedWidth(350);
    setVisible(false);

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);

    // Header
    QWidget *headerRow = new QWidget(this);
    QHBoxLayout *headerLayout = new QHBoxLayout(headerRow);
    headerLayout->setContentsMargins(12, 12, 12, 0);
    m_header = new QLabel(QStringLiteral("引用来源"), headerRow);
    m_header->setStyleSheet(QStringLiteral("font-weight: 600; font-size: 14px;"));
    m_closeBtn = new QPushButton(QStringLiteral("✕"), headerRow);
    m_closeBtn->setFlat(true);
    m_closeBtn->setCursor(Qt::PointingHandCursor);
    m_closeBtn->setFixedSize(24, 24);
    headerLayout->addWidget(m_header);
    headerLayout->addStretch();
    headerLayout->addWidget(m_closeBtn);

    layout->addWidget(headerRow);

    // Scroll area
    m_scroll = new QScrollArea(this);
    m_scroll->setWidgetResizable(true);
    m_scroll->setFrameShape(QFrame::NoFrame);
    m_container = new QWidget();
    m_listLayout = new QVBoxLayout(m_container);
    m_listLayout->setContentsMargins(8, 8, 8, 8);
    m_listLayout->setSpacing(6);
    m_listLayout->addStretch();
    m_scroll->setWidget(m_container);
    layout->addWidget(m_scroll, 1);

    connect(m_closeBtn, &QPushButton::clicked, this, &SourcesSidebar::close);

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void SourcesSidebar::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "SourcesSidebar { background-color: %1; border-left: 1px solid %2; }"
    ).arg(ThemeColors::windowBackground(dark).name(),
          ThemeColors::border(dark).name()));
}

bool SourcesSidebar::isOpen() const { return m_open; }

void SourcesSidebar::setSources(const QJsonArray &sources)
{
    m_sources = combineLikeSources(sources);
    rebuild();
}

QJsonArray SourcesSidebar::combineLikeSources(const QJsonArray &sources) const
{
    QJsonArray deduped;
    QSet<QString> seen;
    for (const QJsonValue &v : sources) {
        QJsonObject src = v.toObject();
        QString key = src.value(QStringLiteral("title")).toString(
            src.value(QStringLiteral("name")).toString());
        if (key.isEmpty() || seen.contains(key))
            continue;
        seen.insert(key);
        deduped.append(src);
    }
    return deduped;
}

void SourcesSidebar::rebuild()
{
    // Clear existing items (but keep the trailing stretch)
    while (m_listLayout->count() > 1) {
        QLayoutItem *item = m_listLayout->takeAt(0);
        delete item->widget();
        delete item;
    }

    for (const QJsonValue &v : m_sources) {
        SourceItem *item = new SourceItem(v.toObject(), m_container);
        connect(item, &SourceItem::clicked, this, [this](const QJsonObject &src) {
            CitationDetailModal *modal = new CitationDetailModal(src, this->window());
            modal->setAttribute(Qt::WA_DeleteOnClose);
            modal->show();
        });
        m_listLayout->insertWidget(m_listLayout->count() - 1, item);
    }
}

void SourcesSidebar::open()
{
    m_open = true;
    show();
    raise();
}

void SourcesSidebar::close()
{
    m_open = false;
    hide();
    emit closeRequested();
}

void SourcesSidebar::clear()
{
    m_sources = QJsonArray();
    rebuild();
}
