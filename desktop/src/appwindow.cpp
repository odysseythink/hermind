#include "appwindow.h"
#include "topbar.h"
#include "sessionlistwidget.h"
#include "chatwidget.h"
#include "statusfooter.h"
#include "httplib.h"
#include "settingsdialog.h"

#include <QVBoxLayout>
#include <QCloseEvent>

AppWindow::AppWindow(QWidget *parent)
    : QWidget(parent),
      m_topBar(nullptr),
      m_splitter(new QSplitter(this)),
      m_sessionList(new SessionListWidget(this)),
      m_chatWidget(new ChatWidget(this)),
      m_footer(new StatusFooter(this)),
      m_settingsDialog(nullptr)
{
    setWindowTitle("hermind");
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
        if (mode == "settings") {
            if (!m_settingsDialog) {
                m_settingsDialog = new SettingsDialog(m_chatWidget->client(), this);
            }
            m_settingsDialog->exec();
            // Reset mode back to chat after dialog closes
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

void AppWindow::closeEvent(QCloseEvent *event)
{
    event->ignore();
    hide();
}
