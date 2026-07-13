#include "suggested_messages_editor.h"
#include "icon_button.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QVBoxLayout>

SuggestedMessagesEditor::SuggestedMessagesEditor(QWidget *parent)
    : QWidget(parent)
{
    auto *rootLayout = new QVBoxLayout(this);
    rootLayout->setContentsMargins(0, 0, 0, 0);
    rootLayout->setSpacing(12);

    auto *titleLabel = new QLabel(tr("Suggested chat messages"), this);
    titleLabel->setStyleSheet(QStringLiteral("font-size: 14px; font-weight: 600;"));
    rootLayout->addWidget(titleLabel);

    auto *descLabel = new QLabel(tr("Up to 4 messages that users can click to start a chat."), this);
    descLabel->setStyleSheet(QStringLiteral("font-size: 12px;"));
    descLabel->setWordWrap(true);
    rootLayout->addWidget(descLabel);

    auto *rowsContainer = new QWidget(this);
    rowsContainer->setObjectName(QStringLiteral("rowsContainer"));
    auto *rowsLayout = new QVBoxLayout(rowsContainer);
    rowsLayout->setContentsMargins(0, 0, 0, 0);
    rowsLayout->setSpacing(8);
    rootLayout->addWidget(rowsContainer);

    m_addButton = new QPushButton(tr("+ Add message"), this);
    m_addButton->setObjectName(QStringLiteral("addMessageButton"));
    connect(m_addButton, &QPushButton::clicked,
            this, &SuggestedMessagesEditor::addMessage);
    rootLayout->addWidget(m_addButton);

    m_saveButton = new QPushButton(tr("Save suggestions"), this);
    m_saveButton->setObjectName(QStringLiteral("saveSuggestionsButton"));
    m_saveButton->setVisible(false);
    connect(m_saveButton, &QPushButton::clicked,
            this, &SuggestedMessagesEditor::onSaveClicked);
    rootLayout->addWidget(m_saveButton);

    rootLayout->addStretch();

    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, [this](const QString &) { rebuild(); });
}

QStringList SuggestedMessagesEditor::messages() const
{
    QStringList list;
    for (const Row &row : m_rows)
        list.append(row.edit->text());
    return list;
}

QStringList SuggestedMessagesEditor::validMessages() const
{
    QStringList list;
    for (const Row &row : m_rows) {
        const QString text = row.edit->text().trimmed();
        if (!text.isEmpty())
            list.append(text);
    }
    return list;
}

bool SuggestedMessagesEditor::hasChanges() const
{
    return m_hasChanges;
}

void SuggestedMessagesEditor::setMessages(const QStringList &messages)
{
    m_initialMessages = messages;
    rebuild();
}

void SuggestedMessagesEditor::markSaved()
{
    m_initialMessages = messages();
    updateChangeState();
}

void SuggestedMessagesEditor::addMessage()
{
    if (m_rows.size() >= kMaxMessages) {
        m_addButton->setEnabled(false);
        return;
    }

    QStringList current = messages();
    current.append(QString());
    m_initialMessages = current; // do not treat newly added empty row as change yet
    rebuild();

    if (!m_rows.isEmpty())
        m_rows.last().edit->setFocus();
    updateChangeState();
}

void SuggestedMessagesEditor::removeRow()
{
    auto *btn = qobject_cast<QAbstractButton *>(sender());
    if (!btn)
        return;

    for (int i = 0; i < m_rows.size(); ++i) {
        if (m_rows.at(i).container->findChild<QAbstractButton *>(QStringLiteral("removeMessageButton")) == btn) {
            m_rows.removeAt(i);
            m_initialMessages.removeAt(i);
            rebuild();
            updateChangeState();
            return;
        }
    }
}

void SuggestedMessagesEditor::onTextChanged()
{
    updateChangeState();
}

void SuggestedMessagesEditor::onSaveClicked()
{
    emit saveRequested();
}

void SuggestedMessagesEditor::rebuild()
{
    QWidget *container = findChild<QWidget *>(QStringLiteral("rowsContainer"));
    if (!container)
        return;

    QLayoutItem *child;
    while ((child = container->layout()->takeAt(0)) != nullptr) {
        if (child->widget())
            delete child->widget();
        delete child;
    }
    m_rows.clear();

    const bool dark = ThemeManager::instance().isDarkMode();
    const QString textColor = ThemeColors::textPrimary(dark).name();
    const QString borderColor = ThemeColors::border(dark).name();

    for (int i = 0; i < m_initialMessages.size(); ++i) {
        auto *rowWidget = new QWidget(container);
        auto *rowLayout = new QHBoxLayout(rowWidget);
        rowLayout->setContentsMargins(0, 0, 0, 0);
        rowLayout->setSpacing(8);

        auto *edit = new QLineEdit(m_initialMessages.at(i), rowWidget);
        edit->setObjectName(QStringLiteral("messageEdit_%1").arg(i));
        edit->setPlaceholderText(tr("Suggested message"));
        edit->setStyleSheet(QStringLiteral(
            "QLineEdit { color: %1; border: 1px solid %2; border-radius: 8px; padding: 8px 12px; }"
        ).arg(textColor, borderColor));
        connect(edit, &QLineEdit::textChanged,
                this, &SuggestedMessagesEditor::onTextChanged);
        rowLayout->addWidget(edit, 1);

        auto *removeBtn = new IconButton(rowWidget);
        removeBtn->setObjectName(QStringLiteral("removeMessageButton"));
        removeBtn->setIconText(QStringLiteral("×"));
        removeBtn->setToolTip(tr("Remove"));
        connect(removeBtn, &IconButton::clicked,
                this, &SuggestedMessagesEditor::removeRow);
        rowLayout->addWidget(removeBtn);

        container->layout()->addWidget(rowWidget);

        Row row;
        row.container = rowWidget;
        row.edit = edit;
        row.originalText = m_initialMessages.at(i);
        m_rows.append(row);
    }

    updateAddButton();
}

void SuggestedMessagesEditor::updateChangeState()
{
    const QStringList current = messages();
    m_hasChanges = (current != m_initialMessages);
    m_saveButton->setVisible(m_hasChanges);
}

void SuggestedMessagesEditor::updateAddButton()
{
    m_addButton->setEnabled(m_rows.size() <= kMaxMessages);
}
