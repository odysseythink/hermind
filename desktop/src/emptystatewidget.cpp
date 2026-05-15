#include "emptystatewidget.h"

#include <QLabel>
#include <QPushButton>
#include <QVBoxLayout>
#include <QHBoxLayout>

EmptyStateWidget::EmptyStateWidget(QWidget *parent)
    : QWidget(parent)
{
    setupUI();
}

void EmptyStateWidget::setupUI()
{
    QVBoxLayout *mainLayout = new QVBoxLayout(this);
    mainLayout->setContentsMargins(0, 0, 0, 0);
    mainLayout->addStretch(1);

    QLabel *title = new QLabel("How can I help you today?", this);
    title->setStyleSheet(
        "font-size: 28px; font-weight: 600; color: #e8e6e3;"
    );
    title->setAlignment(Qt::AlignCenter);

    mainLayout->addWidget(title, 0, Qt::AlignCenter);

    QHBoxLayout *suggestionLayout = new QHBoxLayout;
    suggestionLayout->setSpacing(12);

    QStringList suggestions = {
        "Explain a concept",
        "Write some code",
        "Debug an error"
    };

    for (const QString &text : suggestions) {
        QPushButton *btn = new QPushButton(text, this);
        btn->setStyleSheet(
            "QPushButton { background: #14161a; color: #8a8680; "
            "border: 1px solid #2a2e36; border-radius: 8px; padding: 10px 20px; "
            "font-size: 13px; }"
            "QPushButton:hover { border-color: #FFB800; color: #e8e6e3; }"
        );
        connect(btn, &QPushButton::clicked, this, [this, text]() {
            emit suggestionClicked(text);
        });
        suggestionLayout->addWidget(btn);
    }

    mainLayout->addLayout(suggestionLayout);
    mainLayout->addStretch(1);
}
