#include "prompt_input.h"
#include "agent_menu.h"
#include "tools_menu.h"
#include "attach_item.h"
#include "attachment_manager.h"
#include "theme_manager.h"
#include "theme_colors.h"
#include "hermind_api_client.h"

#include <QTextEdit>
#include <QPushButton>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QKeyEvent>
#include <QDragEnterEvent>
#include <QDropEvent>
#include <QMimeData>
#include <QUrl>

PromptInput::PromptInput(QWidget *parent)
    : QWidget(parent)
{
    QVBoxLayout *root = new QVBoxLayout(this);
    root->setContentsMargins(0, 0, 0, 0);
    root->setSpacing(0);

    m_textEdit = new QTextEdit(this);
    m_textEdit->setPlaceholderText(QStringLiteral("发送消息"));
    m_textEdit->setAcceptRichText(false);
    m_textEdit->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_textEdit->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_textEdit->setFrameShape(QFrame::NoFrame);
    m_textEdit->document()->setDocumentMargin(8);
    m_textEdit->setTabChangesFocus(false);
    m_textEdit->setAcceptDrops(true); // drag-drop handled in eventFilter

    m_agentButton = new QPushButton(QStringLiteral("@"), this);
    m_agentButton->setFixedWidth(28);
    m_agentButton->setFixedHeight(28);
    m_agentButton->setCursor(Qt::PointingHandCursor);
    m_agentButton->setToolTip(QStringLiteral("启动 Agent 会话"));
    m_agentButton->setFlat(true);

    m_toolsButton = new QPushButton(QStringLiteral("工具"), this);
    m_toolsButton->setCursor(Qt::PointingHandCursor);
    m_toolsButton->setToolTip(QStringLiteral("工具与 Slash 命令"));
    m_toolsButton->setFlat(true);

    m_sendButton = new QPushButton(QStringLiteral("发送"), this);
    m_stopButton = new QPushButton(QStringLiteral("停止"), this);
    m_stopButton->setVisible(false);

    m_attachItem = new AttachItem(this);

    QHBoxLayout *btnLayout = new QHBoxLayout();
    btnLayout->addWidget(m_attachItem);
    btnLayout->addWidget(m_agentButton);
    btnLayout->addWidget(m_toolsButton);
    btnLayout->addStretch();
    btnLayout->addWidget(m_sendButton);
    btnLayout->addWidget(m_stopButton);

    m_attachManager = new AttachmentManager(this);
    m_attachManager->setVisible(false);

    root->addWidget(m_attachManager);
    root->addWidget(m_textEdit, 1);
    root->addLayout(btnLayout);

    connect(m_attachItem, &AttachItem::filesSelected,
            m_attachManager, &AttachmentManager::addFiles);
    connect(m_attachManager, &AttachmentManager::processingChanged,
            this, [this](bool) { updateSendEnabled(); });

    connect(m_textEdit, &QTextEdit::textChanged, this, &PromptInput::onTextChanged);
    connect(m_sendButton, &QPushButton::clicked, this, &PromptInput::sendCurrent);
    connect(m_stopButton, &QPushButton::clicked, this, &PromptInput::stopRequested);

    m_agentMenu = new AgentMenu(nullptr); // top-level popup
    m_agentMenu->setSendCommandCallback([this](const QString &text, const QString &mode) {
        PromptCommand cmd;
        cmd.text = text;
        cmd.writeMode = mode;
        emit sendCommand(cmd);
    });
    connect(m_agentButton, &QPushButton::clicked, this, [this]() {
        QPoint globalPos = m_agentButton->mapToGlobal(
            QPoint(m_agentButton->width() / 2, m_agentButton->height()));
        m_agentMenu->showAt(globalPos);
    });

    m_toolsMenu = new ToolsMenu(nullptr); // top-level popup
    m_toolsMenu->setSendCommandCallback([this](const QString &text, const QString &mode) {
        PromptCommand cmd;
        cmd.text = text;
        cmd.writeMode = mode;
        emit sendCommand(cmd);
    });
    connect(m_toolsButton, &QPushButton::clicked, this, [this]() {
        m_autoOpenedTools = false;
        if (m_toolsMenu->isVisible())
            m_toolsMenu->hide();
        else
            m_toolsMenu->showAbove(m_textEdit);
    });
    connect(m_toolsMenu, &ToolsMenu::closed, this, [this]() {
        m_textEdit->setFocus();
    });

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { applyTheme(); });
    applyTheme();

    // Enter/Shift+Enter must be handled on the text edit itself; key events
    // posted to m_textEdit never reach this widget's keyPressEvent().
    m_textEdit->installEventFilter(this);
}

QString PromptInput::text() const
{
    return m_textEdit->toPlainText().trimmed();
}

void PromptInput::setText(const QString &text)
{
    m_textEdit->setPlainText(text);
}

void PromptInput::clear()
{
    m_textEdit->clear();
}

void PromptInput::setPlaceholderText(const QString &text)
{
    m_textEdit->setPlaceholderText(text);
}

void PromptInput::setMaxHeight(int px)
{
    m_maxHeight = px;
}

int PromptInput::maxHeight() const
{
    return m_maxHeight;
}

void PromptInput::setSendEnabled(bool enabled)
{
    m_sendEnabled = enabled;
    updateSendEnabled();
}

bool PromptInput::isSendEnabled() const
{
    return m_sendButton->isEnabled();
}

void PromptInput::updateSendEnabled()
{
    m_sendButton->setEnabled(m_sendEnabled && !m_attachManager->isProcessing());
}

void PromptInput::setStopVisible(bool visible)
{
    m_stopButton->setVisible(visible);
}

void PromptInput::setApiClient(HermindApiClient *client)
{
    m_attachManager->setApiClient(client);
}

void PromptInput::setWorkspaceSlug(const QString &slug)
{
    m_attachManager->setWorkspaceSlug(slug);
}

bool PromptInput::isProcessingAttachments() const
{
    return m_attachManager->isProcessing();
}

QTextEdit *PromptInput::textEdit() const
{
    return m_textEdit;
}

QStringList PromptInput::attachments() const
{
    return m_attachManager->filePaths();
}

bool PromptInput::eventFilter(QObject *obj, QEvent *event)
{
    if (obj == m_textEdit
        && (event->type() == QEvent::DragEnter || event->type() == QEvent::DragMove)) {
        auto *dragEvent = static_cast<QDragMoveEvent *>(event);
        if (dragEvent->mimeData()->hasUrls()) {
            dragEvent->acceptProposedAction();
            return true;
        }
    }
    if (obj == m_textEdit && event->type() == QEvent::Drop) {
        auto *dropEvent = static_cast<QDropEvent *>(event);
        QStringList paths;
        for (const QUrl &url : dropEvent->mimeData()->urls()) {
            if (url.isLocalFile())
                paths.append(url.toLocalFile());
        }
        if (!paths.isEmpty()) {
            m_attachManager->addFiles(paths);
            dropEvent->accept();
            return true;
        }
    }
    if (obj == m_textEdit && event->type() == QEvent::KeyPress) {
        QKeyEvent *keyEvent = static_cast<QKeyEvent *>(event);
        if (keyEvent->key() == Qt::Key_Slash
            && !(keyEvent->modifiers() & (Qt::ControlModifier | Qt::MetaModifier))
            && m_textEdit->toPlainText().trimmed().isEmpty()) {
            m_autoOpenedTools = true;
            m_toolsMenu->showAbove(m_textEdit);
            return true; // swallow the "/" character
        }
        if (keyEvent->key() == Qt::Key_Return || keyEvent->key() == Qt::Key_Enter) {
            if (!(keyEvent->modifiers() & Qt::ShiftModifier)) {
                keyEvent->accept();
                sendCurrent();
                return true;
            }
            // Shift+Enter falls through: QTextEdit inserts a newline.
        }
    }
    return QWidget::eventFilter(obj, event);
}

void PromptInput::onTextChanged()
{
    adjustHeight();
}

void PromptInput::adjustHeight()
{
    QTextEdit *edit = m_textEdit;
    int docHeight = static_cast<int>(edit->document()->size().height());
    int margins = static_cast<int>(edit->document()->documentMargin() * 2);
    int frameWidth = static_cast<int>(edit->frameWidth() * 2);
    int target = qMin(docHeight + margins + frameWidth + 4, m_maxHeight);
    edit->setFixedHeight(target);
    edit->setMinimumHeight(40);
}

void PromptInput::sendCurrent()
{
    const QString content = text();
    if (content.isEmpty() && m_attachManager->count() == 0)
        return;
    // Wait for upload+embed to finish; embedded documents reach the LLM
    // through workspace RAG context, images ride along as data URLs.
    if (m_attachManager->isProcessing())
        return;

    PromptCommand cmd;
    cmd.text = content;
    cmd.writeMode = QStringLiteral("replace");
    cmd.attachments = m_attachManager->imageDataUrls();
    emit sendCommand(cmd);
    m_attachManager->clear();
}

void PromptInput::applyTheme()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    QString style = QStringLiteral(
        "PromptInput { background-color: %1; border: 1px solid %2; border-radius: 16px; }"
    ).arg(ThemeColors::windowBackground(dark).name(),
          ThemeColors::hoverBackground(dark).name());
    setStyleSheet(style);
}
