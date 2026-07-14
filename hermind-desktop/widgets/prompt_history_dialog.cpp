#include "prompt_history_dialog.h"

#include "api_response.h"
#include "hermind_api_client.h"

#include <QHBoxLayout>
#include <QJsonObject>
#include <QJsonValue>
#include <QLabel>
#include <QMessageBox>
#include <QPushButton>
#include <QScrollArea>
#include <QVBoxLayout>

PromptHistoryDialog::PromptHistoryDialog(HermindApiClient *apiClient,
                                         const QString &workspaceSlug,
                                         QWidget *parent)
    : QDialog(parent)
    , m_apiClient(apiClient)
    , m_workspaceSlug(workspaceSlug)
{
    setObjectName(QStringLiteral("promptHistoryDialog"));
    setWindowTitle(tr("Prompt History"));
    resize(560, 420);

    auto *rootLayout = new QVBoxLayout(this);

    auto *scrollArea = new QScrollArea(this);
    scrollArea->setWidgetResizable(true);
    auto *container = new QWidget(scrollArea);
    m_listLayout = new QVBoxLayout(container);
    m_listLayout->setContentsMargins(4, 4, 4, 4);
    m_listLayout->addStretch();
    scrollArea->setWidget(container);
    rootLayout->addWidget(scrollArea);

    auto *clearButton = new QPushButton(tr("Clear All History"), this);
    clearButton->setObjectName(QStringLiteral("clearHistoryButton"));
    connect(clearButton, &QPushButton::clicked, this, &PromptHistoryDialog::clearAll);
    rootLayout->addWidget(clearButton, 0, Qt::AlignLeft);

    loadHistory();
}

void PromptHistoryDialog::loadHistory()
{
    m_apiClient->promptHistory(m_workspaceSlug,
        [this](const QJsonArray &history, const ApiError &error) {
            if (!error.isEmpty()) {
                QMessageBox::warning(this, tr("Load failed"), error.message());
                return;
            }
            renderHistory(history);
        });
}

void PromptHistoryDialog::renderHistory(const QJsonArray &history)
{
    // Clear existing rows (keep the trailing stretch).
    while (m_listLayout->count() > 1) {
        QLayoutItem *item = m_listLayout->takeAt(0);
        if (item->widget())
            item->widget()->deleteLater();
        delete item;
    }

    if (history.isEmpty()) {
        auto *emptyLabel = new QLabel(tr("No prompt history yet."), this);
        emptyLabel->setObjectName(QStringLiteral("promptHistoryEmptyLabel"));
        m_listLayout->insertWidget(0, emptyLabel);
        return;
    }

    int row = 0;
    for (const QJsonValue &value : history) {
        const QJsonObject entry = value.toObject();
        const int id = entry.value(QStringLiteral("id")).toInt();
        const QString prompt = entry.value(QStringLiteral("prompt")).toString();
        const QString modifiedAt = entry.value(QStringLiteral("modifiedAt")).toString();

        QString preview = prompt;
        if (preview.size() > 200)
            preview = preview.left(200) + QStringLiteral("\u2026");

        auto *rowWidget = new QWidget(this);
        auto *rowLayout = new QHBoxLayout(rowWidget);
        rowLayout->setContentsMargins(0, 0, 0, 0);

        auto *textLayout = new QVBoxLayout();
        auto *promptLabel = new QLabel(preview, rowWidget);
        promptLabel->setWordWrap(true);
        auto *dateLabel = new QLabel(modifiedAt, rowWidget);
        dateLabel->setStyleSheet(QStringLiteral("color: gray;"));
        textLayout->addWidget(promptLabel);
        textLayout->addWidget(dateLabel);
        rowLayout->addLayout(textLayout, 1);

        auto *restoreButton = new QPushButton(tr("Restore"), rowWidget);
        restoreButton->setObjectName(QStringLiteral("restorePromptButton_%1").arg(id));
        connect(restoreButton, &QPushButton::clicked, this, [this, prompt]() {
            emit promptSelected(prompt);
            accept();
        });
        rowLayout->addWidget(restoreButton);

        auto *deleteButton = new QPushButton(tr("Delete"), rowWidget);
        deleteButton->setObjectName(QStringLiteral("deletePromptButton_%1").arg(id));
        connect(deleteButton, &QPushButton::clicked, this, [this, id]() {
            deleteItem(id);
        });
        rowLayout->addWidget(deleteButton);

        m_listLayout->insertWidget(row++, rowWidget);
    }
}

void PromptHistoryDialog::deleteItem(int id)
{
    if (QMessageBox::question(this, tr("Delete prompt"),
                              tr("Delete this prompt from history?"))
        != QMessageBox::Yes)
        return;

    m_apiClient->deletePromptHistoryItem(m_workspaceSlug, id,
        [this](bool success, const QString &, const ApiError &error) {
            if (!error.isEmpty() || !success) {
                QMessageBox::warning(this, tr("Delete failed"), error.message());
                return;
            }
            loadHistory();
        });
}

void PromptHistoryDialog::clearAll()
{
    if (QMessageBox::question(this, tr("Clear prompt history"),
                              tr("Delete all prompt history for this workspace?"))
        != QMessageBox::Yes)
        return;

    m_apiClient->clearPromptHistory(m_workspaceSlug,
        [this](bool success, const QString &, const ApiError &error) {
            if (!error.isEmpty() || !success) {
                QMessageBox::warning(this, tr("Clear failed"), error.message());
                return;
            }
            loadHistory();
        });
}
