#include "topbar.h"

#include <QLabel>
#include <QPushButton>
#include <QHBoxLayout>
#include <QSpacerItem>
#include <QButtonGroup>

TopBar::TopBar(QWidget *parent)
    : QWidget(parent)
{
    setFixedHeight(48);
    setupUI();
}

void TopBar::setupUI()
{
    QHBoxLayout *layout = new QHBoxLayout(this);
    layout->setContentsMargins(16, 0, 16, 0);
    layout->setSpacing(12);

    m_brandLabel = new QLabel("◈ HERMIND", this);
    m_brandLabel->setStyleSheet(
        "font-family: monospace; font-size: 14px; font-weight: 600; "
        "text-transform: uppercase; letter-spacing: 0.05em; color: #e8e6e3;"
    );

    layout->addWidget(m_brandLabel);
    layout->addSpacerItem(new QSpacerItem(0, 0, QSizePolicy::Expanding, QSizePolicy::Minimum));

    QButtonGroup *modeGroup = new QButtonGroup(this);
    m_chatModeBtn = new QPushButton("Chat", this);
    m_chatModeBtn->setCheckable(true);
    m_chatModeBtn->setChecked(true);
    m_chatModeBtn->setStyleSheet(
        "QPushButton { background: transparent; color: #8a8680; border: 1px solid #2a2e36; "
        "border-radius: 4px; padding: 4px 12px; font-size: 11px; font-weight: 600; "
        "text-transform: uppercase; font-family: monospace; }"
        "QPushButton:checked { background: #FFB800; color: #0a0b0d; border-color: #FFB800; }"
    );

    m_settingsModeBtn = new QPushButton("Set", this);
    m_settingsModeBtn->setCheckable(true);
    m_settingsModeBtn->setStyleSheet(
        "QPushButton { background: transparent; color: #8a8680; border: 1px solid #2a2e36; "
        "border-radius: 4px; padding: 4px 12px; font-size: 11px; font-weight: 600; "
        "text-transform: uppercase; font-family: monospace; }"
        "QPushButton:checked { background: #FFB800; color: #0a0b0d; border-color: #FFB800; }"
    );

    modeGroup->addButton(m_chatModeBtn);
    modeGroup->addButton(m_settingsModeBtn);
    modeGroup->setExclusive(true);

    layout->addWidget(m_chatModeBtn);
    layout->addWidget(m_settingsModeBtn);

    m_statusDot = new QWidget(this);
    m_statusDot->setFixedSize(8, 8);
    m_statusDot->setStyleSheet("background: #7ee787; border-radius: 4px;");

    m_statusLabel = new QLabel("READY", this);
    m_statusLabel->setStyleSheet(
        "font-family: monospace; font-size: 12px; text-transform: uppercase; color: #8a8680;"
    );

    layout->addWidget(m_statusDot);
    layout->addWidget(m_statusLabel);

    m_saveBtn = new QPushButton("Save", this);
    m_saveBtn->setStyleSheet(
        "QPushButton { background: #FFB800; color: #0a0b0d; border: 1px solid #FFB800; "
        "border-radius: 4px; padding: 4px 14px; font-size: 11px; font-weight: 600; "
        "text-transform: uppercase; font-family: monospace; }"
        "QPushButton:hover { background: #FF8A00; border-color: #FF8A00; }"
    );
    layout->addWidget(m_saveBtn);

    connect(m_chatModeBtn, &QPushButton::clicked, this, [this]() { emit modeChanged("chat"); });
    connect(m_settingsModeBtn, &QPushButton::clicked, this, [this]() { emit modeChanged("settings"); });
    connect(m_saveBtn, &QPushButton::clicked, this, &TopBar::saveRequested);
}

void TopBar::setStatus(const QString &status)
{
    m_statusLabel->setText(status.toUpper());
}

void TopBar::setStatusDotColor(const QString &color)
{
    m_statusDot->setStyleSheet(QString("background: %1; border-radius: 4px;").arg(color));
}
