#include "appwindow.h"
#include "sessionlistwidget.h"
#include "chatwidget.h"
#include "httplib.h"

AppWindow::AppWindow(QWidget *parent)
    : QMainWindow(parent),
      m_splitter(new QSplitter(this)),
      m_sessionList(new SessionListWidget(this)),
      m_chatWidget(new ChatWidget(this))
{
    setWindowTitle("hermind");
    resize(1200, 800);

    m_sessionList->setMinimumWidth(200);
    m_sessionList->setMaximumWidth(400);
    m_sessionList->setSizePolicy(QSizePolicy::Fixed, QSizePolicy::Expanding);

    m_splitter->addWidget(m_sessionList);
    m_splitter->addWidget(m_chatWidget);
    m_splitter->setStretchFactor(0, 0);
    m_splitter->setStretchFactor(1, 1);
    m_splitter->setSizes(QList<int>() << 250 << 950);

    setCentralWidget(m_splitter);

    connect(m_sessionList, &SessionListWidget::sessionSelected,
            m_chatWidget, &ChatWidget::loadSession);
    connect(m_sessionList, &SessionListWidget::newSessionRequested,
            m_chatWidget, &ChatWidget::startNewSession);
}

void AppWindow::setClient(HermindClient *client)
{
    m_chatWidget->setClient(client);
}

void AppWindow::closeEvent(QCloseEvent *event)
{
    QMainWindow::closeEvent(event);
}
