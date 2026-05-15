#include "appwindow.h"
#include "topbar.h"
#include "sessionlistwidget.h"
#include "chatwidget.h"
#include "statusfooter.h"
#include "httplib.h"
#include "settingseditor.h"
#include "thememanager.h"
#include "i18nmanager.h"

#include <QVBoxLayout>
#include <QCloseEvent>

AppWindow::AppWindow(QWidget *parent)
    : QWidget(parent),
      m_topBar(nullptr),
      m_splitter(new QSplitter(this)),
      m_sessionList(new SessionListWidget(this)),
      m_chatWidget(new ChatWidget(this)),
      m_footer(new StatusFooter(this)),
      m_settingsEditor(nullptr),
      m_themeManager(nullptr),
      m_i18nManager(nullptr)
{
    setWindowTitle(QStringLiteral("hermind"));
    resize(1200, 800);

    setupUI();
}

void AppWindow::setupUI()
{
    QVBoxLayout *mainLayout = new QVBoxLayout(this);
    mainLayout->setContentsMargins(0, 0, 0, 0);
    mainLayout->setSpacing(0);

    setupTopBar();
    setupSidebar();
    setupChatArea();
    setupFooter();

    mainLayout->addWidget(m_topBar);
    mainLayout->addWidget(m_splitter, 1);
    mainLayout->addWidget(m_footer);

    connect(m_sessionList, &SessionListWidget::sessionSelected,
            m_chatWidget, &ChatWidget::loadSession);
    connect(m_sessionList, &SessionListWidget::newSessionRequested,
            m_chatWidget, &ChatWidget::startNewSession);
}

void AppWindow::setupTopBar()
{
    m_topBar = new TopBar(this);
    connect(m_topBar, &TopBar::modeChanged, this, [this](const QString &mode) {
        if (mode == QStringLiteral("settings")) {
            if (!m_settingsEditor) {
                m_settingsEditor = new SettingsEditor(m_chatWidget->client(), this);
            }
            m_settingsEditor->exec();
        }
    });
    connect(m_topBar, &TopBar::themeToggled, this, [this]() {
        if (m_themeManager) {
            ThemeManager::Theme next = (m_themeManager->currentTheme() == ThemeManager::Dark)
                                           ? ThemeManager::Light
                                           : ThemeManager::Dark;
            m_themeManager->setTheme(next);
        }
    });
    connect(m_topBar, &TopBar::languageChanged, this, [this](const QString &langCode) {
        if (m_i18nManager) {
            m_i18nManager->setLanguage(langCode);
        }
    });
}

void AppWindow::setupSidebar()
{
    m_sessionList->setMinimumWidth(200);
    m_sessionList->setMaximumWidth(400);
    m_sessionList->setSizePolicy(QSizePolicy::Fixed, QSizePolicy::Expanding);
}

void AppWindow::setupChatArea()
{
    m_splitter->addWidget(m_sessionList);
    m_splitter->addWidget(m_chatWidget);
    m_splitter->setStretchFactor(0, 0);
    m_splitter->setStretchFactor(1, 1);
    m_splitter->setSizes(QList<int>() << 240 << 960);
}

void AppWindow::setupFooter()
{
    // Footer already constructed in initializer list
}

void AppWindow::setClient(HermindClient *client)
{
    m_chatWidget->setClient(client);
}

void AppWindow::setThemeManager(ThemeManager *manager)
{
    m_themeManager = manager;
    if (m_themeManager) {
        m_topBar->setThemeDark(m_themeManager->currentTheme() == ThemeManager::Dark);
        connect(m_themeManager, &ThemeManager::themeChanged, this, [this](ThemeManager::Theme theme) {
            m_topBar->setThemeDark(theme == ThemeManager::Dark);
        });
    }
}

void AppWindow::setI18nManager(I18nManager *manager)
{
    m_i18nManager = manager;
}

void AppWindow::closeEvent(QCloseEvent *event)
{
    event->ignore();
    hide();
}
