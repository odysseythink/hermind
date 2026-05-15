#include "emptystatewidget.h"
#include "httplib.h"

#include <QLabel>
#include <QPushButton>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QDebug>

EmptyStateWidget::EmptyStateWidget(QWidget *parent)
    : QWidget(parent),
      m_client(nullptr),
      m_mainLayout(nullptr),
      m_suggestionLayout(nullptr)
{
    setupUI();
}

void EmptyStateWidget::setupUI()
{
    m_mainLayout = new QVBoxLayout(this);
    m_mainLayout->setContentsMargins(0, 0, 0, 0);
    m_mainLayout->addStretch(1);

    QLabel *title = new QLabel(QStringLiteral("How can I help you today?"), this);
    title->setStyleSheet(
        QStringLiteral("font-size: 28px; font-weight: 600; color: #e8e6e3;")
    );
    title->setAlignment(Qt::AlignCenter);
    m_mainLayout->addWidget(title, 0, Qt::AlignCenter);

    m_suggestionLayout = new QHBoxLayout;
    m_suggestionLayout->setSpacing(12);
    m_mainLayout->addLayout(m_suggestionLayout);
    m_mainLayout->addStretch(1);

    // Default suggestions until backend responds
    buildSuggestionButtons({
        QStringLiteral("Explain a concept"),
        QStringLiteral("Write some code"),
        QStringLiteral("Debug an error")
    });
}

void EmptyStateWidget::setClient(HermindClient *client)
{
    m_client = client;
    refreshSuggestions();
}

void EmptyStateWidget::refreshSuggestions()
{
    if (!m_client)
        return;

    m_client->get(QStringLiteral("/api/suggestions"),
                  [this](const QJsonObject &resp, const QString &error) {
        if (!error.isEmpty()) {
            qWarning() << "Failed to load suggestions:" << error;
            return;
        }
        QJsonArray arr = resp.value(QStringLiteral("suggestions")).toArray();
        if (arr.isEmpty())
            return;

        QStringList suggestions;
        for (const auto &v : arr) {
            suggestions.append(v.toString());
        }
        buildSuggestionButtons(suggestions);
    });
}

void EmptyStateWidget::buildSuggestionButtons(const QStringList &suggestions)
{
    // Clear existing buttons
    while (m_suggestionLayout->count() > 0) {
        QLayoutItem *item = m_suggestionLayout->takeAt(0);
        if (item->widget()) {
            item->widget()->deleteLater();
        }
        delete item;
    }

    for (const QString &text : suggestions) {
        QPushButton *btn = new QPushButton(text, this);
        btn->setStyleSheet(
            QStringLiteral(
                "QPushButton { background: #14161a; color: #8a8680; "
                "border: 1px solid #2a2e36; border-radius: 8px; padding: 10px 20px; "
                "font-size: 13px; }"
                "QPushButton:hover { border-color: #FFB800; color: #e8e6e3; }"
            )
        );
        connect(btn, &QPushButton::clicked, this, [this, text]() {
            emit suggestionClicked(text);
        });
        m_suggestionLayout->addWidget(btn);
    }
}
