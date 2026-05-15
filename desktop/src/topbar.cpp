#include "topbar.h"

#include <QLabel>
#include <QPushButton>
#include <QHBoxLayout>
#include <QSpacerItem>
#include <QButtonGroup>
#include <QComboBox>

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

    m_brandLabel = new QLabel(QStringLiteral("◈ HERMIND"), this);
    m_brandLabel->setStyleSheet(
        QStringLiteral(
            "font-family: monospace; font-size: 14px; font-weight: 600; "
            "text-transform: uppercase; letter-spacing: 0.05em; color: #e8e6e3;"
        )
    );

    layout->addWidget(m_brandLabel);
    layout->addSpacerItem(new QSpacerItem(0, 0, QSizePolicy::Expanding, QSizePolicy::Minimum));

    // Language selector
    m_langCombo = new QComboBox(this);
    m_langCombo->addItem(QStringLiteral("EN"), QStringLiteral("en"));
    m_langCombo->addItem(QStringLiteral("中"), QStringLiteral("zh_CN"));
    m_langCombo->setStyleSheet(
        QStringLiteral(
            "QComboBox { background: #14161a; color: #8a8680; border: 1px solid #2a2e36; "
            "border-radius: 4px; padding: 4px 8px; font-size: 11px; font-weight: 600; }"
            "QComboBox:hover { border-color: #3a3e46; color: #e8e6e3; }"
            "QComboBox::drop-down { border: none; width: 16px; }"
            "QComboBox QAbstractItemView { background: #14161a; color: #e8e6e3; "
            "border: 1px solid #2a2e36; selection-background-color: #2a2e36; }"
        )
    );
    connect(m_langCombo, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, [this](int idx) {
        emit languageChanged(m_langCombo->itemData(idx).toString());
    });
    layout->addWidget(m_langCombo);

    // Theme toggle
    m_themeBtn = new QPushButton(QStringLiteral("🌙"), this);
    m_themeBtn->setFixedSize(28, 28);
    m_themeBtn->setStyleSheet(
        QStringLiteral(
            "QPushButton { background: #14161a; color: #e8e6e3; border: 1px solid #2a2e36; "
            "border-radius: 4px; font-size: 13px; }"
            "QPushButton:hover { background: #2a2e36; }"
        )
    );
    connect(m_themeBtn, &QPushButton::clicked, this, &TopBar::themeToggled);
    layout->addWidget(m_themeBtn);

    // Mode buttons
    QButtonGroup *modeGroup = new QButtonGroup(this);
    m_chatModeBtn = new QPushButton(QStringLiteral("Chat"), this);
    m_chatModeBtn->setCheckable(true);
    m_chatModeBtn->setChecked(true);
    m_chatModeBtn->setStyleSheet(
        QStringLiteral(
            "QPushButton { background: transparent; color: #8a8680; border: 1px solid #2a2e36; "
            "border-radius: 4px; padding: 4px 12px; font-size: 11px; font-weight: 600; "
            "text-transform: uppercase; font-family: monospace; }"
            "QPushButton:checked { background: #FFB800; color: #0a0b0d; border-color: #FFB800; }"
        )
    );

    m_settingsModeBtn = new QPushButton(QStringLiteral("Set"), this);
    m_settingsModeBtn->setCheckable(true);
    m_settingsModeBtn->setStyleSheet(
        QStringLiteral(
            "QPushButton { background: transparent; color: #8a8680; border: 1px solid #2a2e36; "
            "border-radius: 4px; padding: 4px 12px; font-size: 11px; font-weight: 600; "
            "text-transform: uppercase; font-family: monospace; }"
            "QPushButton:checked { background: #FFB800; color: #0a0b0d; border-color: #FFB800; }"
        )
    );

    modeGroup->addButton(m_chatModeBtn);
    modeGroup->addButton(m_settingsModeBtn);
    modeGroup->setExclusive(true);

    layout->addWidget(m_chatModeBtn);
    layout->addWidget(m_settingsModeBtn);

    // Status
    m_statusDot = new QWidget(this);
    m_statusDot->setFixedSize(8, 8);
    m_statusDot->setStyleSheet(QStringLiteral("background: #7ee787; border-radius: 4px;"));

    m_statusLabel = new QLabel(QStringLiteral("READY"), this);
    m_statusLabel->setStyleSheet(
        QStringLiteral(
            "font-family: monospace; font-size: 12px; text-transform: uppercase; color: #8a8680;"
        )
    );

    layout->addWidget(m_statusDot);
    layout->addWidget(m_statusLabel);

    // Save button
    m_saveBtn = new QPushButton(QStringLiteral("Save"), this);
    m_saveBtn->setStyleSheet(
        QStringLiteral(
            "QPushButton { background: #FFB800; color: #0a0b0d; border: 1px solid #FFB800; "
            "border-radius: 4px; padding: 4px 14px; font-size: 11px; font-weight: 600; "
            "text-transform: uppercase; font-family: monospace; }"
            "QPushButton:hover { background: #FF8A00; border-color: #FF8A00; }"
        )
    );
    layout->addWidget(m_saveBtn);

    connect(m_chatModeBtn, &QPushButton::clicked, this, [this]() { emit modeChanged(QStringLiteral("chat")); });
    connect(m_settingsModeBtn, &QPushButton::clicked, this, [this]() { emit modeChanged(QStringLiteral("settings")); });
    connect(m_saveBtn, &QPushButton::clicked, this, &TopBar::saveRequested);
}

void TopBar::setStatus(const QString &status)
{
    m_statusLabel->setText(status.toUpper());
}

void TopBar::setStatusDotColor(const QString &color)
{
    m_statusDot->setStyleSheet(QStringLiteral("background: %1; border-radius: 4px;").arg(color));
}

void TopBar::setLanguage(const QString &langCode)
{
    int idx = m_langCombo->findData(langCode);
    if (idx >= 0 && m_langCombo->currentIndex() != idx) {
        m_langCombo->setCurrentIndex(idx);
    }
}

void TopBar::setThemeDark(bool dark)
{
    m_themeBtn->setText(dark ? QStringLiteral("🌙") : QStringLiteral("☀️"));
}
