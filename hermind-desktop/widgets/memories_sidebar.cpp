#include "memories_sidebar.h"
#include "memory_card.h"
#include "memory_modal.h"
#include "hermind_api_client.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QScrollArea>
#include <QTabBar>

MemoriesSidebar::MemoriesSidebar(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    setFixedWidth(366);
    setVisible(false);

    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);

    // Header
    QWidget *headerRow = new QWidget(this);
    QHBoxLayout *headerLayout = new QHBoxLayout(headerRow);
    QLabel *title = new QLabel(QStringLiteral("记忆"), headerRow);
    title->setStyleSheet(QStringLiteral("font-weight: 600; font-size: 14px;"));
    m_closeBtn = new QPushButton(QStringLiteral("✕"), headerRow);
    m_closeBtn->setFlat(true);
    m_closeBtn->setFixedSize(24, 24);
    headerLayout->addWidget(title);
    headerLayout->addStretch();
    headerLayout->addWidget(m_closeBtn);
    layout->addWidget(headerRow);

    // Tabs
    m_tabBar = new QTabBar(this);
    m_tabBar->addTab(QStringLiteral("工作区"));
    m_tabBar->addTab(QStringLiteral("全局"));
    layout->addWidget(m_tabBar);

    // Create button
    m_createBtn = new QPushButton(QStringLiteral("+ 创建记忆"), this);
    m_createBtn->setFlat(true);
    layout->addWidget(m_createBtn);

    // Scroll
    m_scroll = new QScrollArea(this);
    m_scroll->setWidgetResizable(true);
    m_scroll->setFrameShape(QFrame::NoFrame);
    m_container = new QWidget();
    m_listLayout = new QVBoxLayout(m_container);
    m_listLayout->setContentsMargins(0, 0, 0, 0);
    m_listLayout->setSpacing(6);
    m_listLayout->addStretch();
    m_scroll->setWidget(m_container);
    layout->addWidget(m_scroll, 1);

    connect(m_closeBtn, &QPushButton::clicked, this, &MemoriesSidebar::close);
    connect(m_tabBar, &QTabBar::currentChanged, this, &MemoriesSidebar::rebuildList);
    connect(m_createBtn, &QPushButton::clicked, this, [this]() {
        MemoryModal *modal = new MemoryModal(MemoryModal::Create, QString(), this->window());
        connect(modal, &MemoryModal::submitted, this, [this](const QString &content) {
            if (!m_apiClient || m_workspaceSlug.isEmpty())
                return;
            const QString scope = (m_tabBar->currentIndex() == 0)
                                      ? QStringLiteral("workspace")
                                      : QStringLiteral("global");
            m_apiClient->createMemory(m_workspaceId, content, scope,
                [this](const HermindMemory &, const QString &, const ApiError &err) {
                    if (err.isEmpty())
                        fetchAndRefresh();
                });
        });
        modal->setAttribute(Qt::WA_DeleteOnClose);
        modal->show();
    });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void MemoriesSidebar::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    setStyleSheet(QStringLiteral(
        "MemoriesSidebar { background-color: %1; border-left: 1px solid %2; }"
    ).arg(dark ? QStringLiteral("#18181b") : QStringLiteral("#ffffff"),
          dark ? QStringLiteral("#27272a") : QStringLiteral("#e2e8f0")));
}

bool MemoriesSidebar::isOpen() const { return m_open; }

void MemoriesSidebar::open()
{
    m_open = true;
    show();
    raise();
    // Fetch on every open: setWorkspace() only fetches while already open,
    // so without this the first open always showed a stale/empty list.
    fetchAndRefresh();
}

void MemoriesSidebar::close()
{
    m_open = false;
    hide();
    emit closeRequested();
}

void MemoriesSidebar::setWorkspace(const QString &slug, int workspaceId)
{
    m_workspaceSlug = slug;
    m_workspaceId = workspaceId;
    if (m_open)
        fetchAndRefresh();
}

void MemoriesSidebar::refresh()
{
    if (m_open)
        fetchAndRefresh();
}

void MemoriesSidebar::fetchAndRefresh()
{
    if (!m_apiClient || m_workspaceSlug.isEmpty())
        return;
    m_apiClient->listMemories(m_workspaceSlug,
        [this](const HermindApiClient::MemoriesResult &result, const ApiError &err) {
            if (!err.isEmpty())
                return;
            m_workspaceMemories = result.workspace;
            m_globalMemories = result.global;
            rebuildList();
        });
}

void MemoriesSidebar::rebuildList()
{
    while (m_listLayout->count() > 1) {
        QLayoutItem *item = m_listLayout->takeAt(0);
        delete item->widget();
        delete item;
    }

    const QVector<HermindMemory> &memories =
        (m_tabBar->currentIndex() == 0) ? m_workspaceMemories : m_globalMemories;

    for (const HermindMemory &mem : memories) {
        MemoryCard *card = new MemoryCard(mem, m_container);
        connect(card, &MemoryCard::editRequested, this, [this](const HermindMemory &mem) {
            MemoryModal *modal = new MemoryModal(MemoryModal::Edit, mem.content(), this->window());
            connect(modal, &MemoryModal::submitted, this,
                    [this, id = mem.id()](const QString &content) {
                if (!m_apiClient)
                    return;
                m_apiClient->updateMemory(id, content,
                    [this](const HermindMemory &, const QString &, const ApiError &err) {
                        if (err.isEmpty())
                            fetchAndRefresh();
                    });
            });
            modal->setAttribute(Qt::WA_DeleteOnClose);
            modal->show();
        });
        connect(card, &MemoryCard::deleteRequested, this, [this](int memoryId) {
            if (!m_apiClient)
                return;
            m_apiClient->deleteMemory(memoryId,
                [this](bool, const ApiError &err) {
                    if (err.isEmpty())
                        fetchAndRefresh();
                });
        });
        m_listLayout->insertWidget(m_listLayout->count() - 1, card);
    }
}
