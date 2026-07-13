#include "default_chat_widget.h"
#include "quick_actions.h"
#include "suggested_messages.h"
#include "hermind_api_client.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QVBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QPixmap>

DefaultChatWidget::DefaultChatWidget(HermindApiClient *apiClient, QWidget *parent)
    : QWidget(parent)
    , m_apiClient(apiClient)
{
    QVBoxLayout *layout = new QVBoxLayout(this);
    layout->setAlignment(Qt::AlignCenter);

    // Center content wrapper
    QWidget *center = new QWidget(this);
    QVBoxLayout *centerLayout = new QVBoxLayout(center);
    centerLayout->setAlignment(Qt::AlignCenter);
    centerLayout->setSpacing(16);

    m_logoLabel = new QLabel(center);
    m_logoLabel->setAlignment(Qt::AlignCenter);
    m_logoLabel->setFixedSize(140, 140);
    m_logoLabel->setStyleSheet(QStringLiteral("background-color: transparent;"));
    centerLayout->addWidget(m_logoLabel, 0, Qt::AlignCenter);

    m_greetingLabel = new QLabel(center);
    m_greetingLabel->setAlignment(Qt::AlignCenter);
    m_greetingLabel->setStyleSheet(QStringLiteral("font-size: 20px; font-weight: 600;"));
    centerLayout->addWidget(m_greetingLabel);

    m_workspaceButton = new QPushButton(center);
    m_workspaceButton->setCursor(Qt::PointingHandCursor);
    m_workspaceButton->setFlat(true);
    m_workspaceButton->setVisible(false);
    centerLayout->addWidget(m_workspaceButton, 0, Qt::AlignCenter);

    m_promptInput = new PromptInput(center);
    m_promptInput->setMaxHeight(200);
    m_promptInput->setMaximumWidth(750);
    centerLayout->addWidget(m_promptInput, 0, Qt::AlignCenter);

    m_quickActions = new QuickActions(center);
    // Quick actions target workspace-settings pages (Phase 2) and have no
    // frontend equivalent; keep them hidden rather than clickable no-ops.
    m_quickActions->setVisible(false);
    centerLayout->addWidget(m_quickActions, 0, Qt::AlignCenter);

    m_suggestedMsgs = new SuggestedMessages(center);
    centerLayout->addWidget(m_suggestedMsgs, 0, Qt::AlignCenter);

    layout->addWidget(center, 0, Qt::AlignCenter);

    connect(m_workspaceButton, &QPushButton::clicked, this, [this]() {
        if (!m_workspaceSlug.isEmpty())
            emit workspaceSelected(m_workspaceSlug);
    });

    connect(m_quickActions, &QuickActions::createAgentClicked, this, &DefaultChatWidget::createAgentClicked);
    connect(m_quickActions, &QuickActions::editWorkspaceClicked, this, &DefaultChatWidget::editWorkspaceClicked);
    connect(m_quickActions, &QuickActions::uploadDocumentClicked, this, &DefaultChatWidget::uploadDocumentClicked);

    m_suggestedMsgs->setSendCommandCallback([this](const QString &text, const QString &) {
        m_promptInput->setText(text);
        emit sendRequested(text);
    });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();
}

void DefaultChatWidget::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString bg = ThemeColors::windowBackground(dark).name();
    const QString fg = ThemeColors::textPrimary(dark).name();
    setStyleSheet(QStringLiteral(
        "DefaultChatWidget { background-color: %1; }"
        "QLabel { color: %2; }"
    ).arg(bg, fg));
}

void DefaultChatWidget::setUsername(const QString &username)
{
    if (username.isEmpty()) {
        // Single-user mode has no username; avoid a dangling "欢迎回来, !".
        m_greetingLabel->setText(QStringLiteral("欢迎回来!"));
        return;
    }
    m_greetingLabel->setText(QStringLiteral("欢迎回来, %1!").arg(username));
}

void DefaultChatWidget::setLogoPath(const QString &path)
{
    QPixmap pix(path);
    if (!pix.isNull())
        m_logoLabel->setPixmap(pix.scaled(140, 140, Qt::KeepAspectRatio, Qt::SmoothTransformation));
}

void DefaultChatWidget::setWorkspaceSlug(const QString &slug)
{
    m_workspaceSlug = slug;
    m_workspaceButton->setText(QStringLiteral("进入 %1 →").arg(slug));
    m_workspaceButton->setVisible(!slug.isEmpty());
}

void DefaultChatWidget::setSuggestedMessages(const QStringList &messages)
{
    m_suggestedMsgs->setMessages(messages);
}

PromptInput *DefaultChatWidget::promptInput() const
{
    return m_promptInput;
}
