#include "setting_row.h"
#include "theme_colors.h"
#include "theme_style_helper.h"

#include <QVBoxLayout>
#include <QHBoxLayout>

SettingRow::SettingRow(QWidget *parent)
    : QWidget(parent)
    , m_titleLabel(new QLabel(this))
    , m_descLabel(new QLabel(this))
{
    m_titleLabel->setWordWrap(false);
    m_descLabel->setWordWrap(true);
    m_descLabel->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Preferred);

    new ThemeStyleHelper(this, [](QWidget *w, bool dark) {
        auto *row = qobject_cast<SettingRow *>(w);
        if (row)
            row->applyStyle(dark);
    }, this);

    rebuildLayout();
}

void SettingRow::setTitle(const QString &title)
{
    m_titleLabel->setText(title);
}

void SettingRow::setDescription(const QString &description)
{
    m_descLabel->setText(description);
}

void SettingRow::setControl(QWidget *control)
{
    if (m_control) {
        m_control->hide();
        m_control->setParent(nullptr);
    }
    m_control = control;
    if (m_control)
        m_control->setParent(this);
    rebuildLayout();
}

void SettingRow::applyStyle(bool dark)
{
    const QString titleColor = ThemeColors::textPrimary(dark).name();
    const QString descColor = ThemeColors::textSecondary(dark).name();

    m_titleLabel->setStyleSheet(QStringLiteral(
        "QLabel { color: %1; font-size: 14px; font-weight: 500; }"
    ).arg(titleColor));

    m_descLabel->setStyleSheet(QStringLiteral(
        "QLabel { color: %1; font-size: 12px; }"
    ).arg(descColor));
}

void SettingRow::rebuildLayout()
{
    QLayout *old = layout();
    if (old)
        delete old;

    auto *main = new QHBoxLayout(this);
    main->setSpacing(16);
    main->setContentsMargins(0, 8, 0, 8);

    auto *textLayout = new QVBoxLayout();
    textLayout->setSpacing(4);
    textLayout->setContentsMargins(0, 0, 0, 0);
    textLayout->addWidget(m_titleLabel);
    textLayout->addWidget(m_descLabel);
    textLayout->addStretch();

    main->addLayout(textLayout, 1);
    if (m_control)
        main->addWidget(m_control, 0, Qt::AlignVCenter);
}
