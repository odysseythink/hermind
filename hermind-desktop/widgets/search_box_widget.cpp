#include "search_box_widget.h"
#include "search_input.h"
#include "theme_colors.h"
#include "theme_manager.h"

#include <QHBoxLayout>
#include <QVBoxLayout>
#include <QLabel>
#include <QFrame>
#include <QMouseEvent>
#include <QApplication>
#include <QScreen>

namespace {

constexpr int kDebounceMs = 500;
constexpr int kMinSearchTermLength = 3;
constexpr int kPopupMaxHeight = 400;

class SearchResultItem : public QWidget
{
    Q_OBJECT
public:
    SearchResultItem(const QString &name, const QString &hint,
                     const QString &workspaceSlug, const QString &threadSlug,
                     QWidget *parent = nullptr)
        : QWidget(parent)
        , m_workspaceSlug(workspaceSlug)
        , m_threadSlug(threadSlug)
    {
        auto *layout = new QHBoxLayout(this);
        layout->setContentsMargins(8, 4, 8, 4);
        layout->setSpacing(4);

        auto *nameLabel = new QLabel(this);
        nameLabel->setWordWrap(false);
        QString text = name.toHtmlEscaped();
        if (!hint.isEmpty()) {
            text += QStringLiteral(" <span style='color: %1; font-size: 11px;'>| %2</span>")
                        .arg(ThemeColors::textSecondary(ThemeManager::instance().isDarkMode()).name(),
                             hint.toHtmlEscaped());
        }
        nameLabel->setText(text);
        layout->addWidget(nameLabel, 1);

        setCursor(Qt::PointingHandCursor);
        applyStyle();
        connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
                this, &SearchResultItem::applyStyle);
    }

    QString workspaceSlug() const { return m_workspaceSlug; }
    QString threadSlug() const { return m_threadSlug; }

signals:
    void clicked(const QString &workspaceSlug, const QString &threadSlug);

protected:
    void mousePressEvent(QMouseEvent *event) override
    {
        if (event->button() == Qt::LeftButton)
            emit clicked(m_workspaceSlug, m_threadSlug);
        QWidget::mousePressEvent(event);
    }
    void enterEvent(QEnterEvent *event) override
    {
        m_hovered = true;
        applyStyle();
        QWidget::enterEvent(event);
    }
    void leaveEvent(QEvent *event) override
    {
        m_hovered = false;
        applyStyle();
        QWidget::leaveEvent(event);
    }

private slots:
    void applyStyle()
    {
        const bool dark = ThemeManager::instance().isDarkMode();
        const QString bg = m_hovered
                               ? ThemeColors::hoverBackground(dark).name()
                               : QStringLiteral("transparent");
        const QString text = ThemeColors::textPrimary(dark).name();
        setStyleSheet(QStringLiteral(
            "SearchResultItem { background-color: %1; border-radius: 4px; }"
            "QLabel { color: %2; font-size: 13px; }"
        ).arg(bg, text));
    }

private:
    QString m_workspaceSlug;
    QString m_threadSlug;
    bool m_hovered = false;
};

} // namespace

SearchBoxWidget::SearchBoxWidget(QWidget *parent)
    : QWidget(parent)
    , m_input(new SearchInput(this))
    , m_popup(new QFrame(nullptr))
    , m_popupLayout(new QVBoxLayout(m_popup))
    , m_statusLabel(new QLabel(m_popup))
    , m_debounceTimer(new QTimer(this))
{
    auto *layout = new QHBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(0);
    layout->addWidget(m_input);

    m_input->setClearButtonEnabled(true);
    m_input->installEventFilter(this);
    connect(m_input, &QLineEdit::textChanged, this, &SearchBoxWidget::onTextChanged);

    m_debounceTimer->setSingleShot(true);
    connect(m_debounceTimer, &QTimer::timeout, this, &SearchBoxWidget::onSearchFinished);

    m_popup->setWindowFlags(Qt::Popup | Qt::FramelessWindowHint | Qt::NoDropShadowWindowHint);
    m_popup->setAttribute(Qt::WA_TranslucentBackground);
    m_popup->setFrameShape(QFrame::StyledPanel);
    m_popup->setMinimumWidth(220);

    m_popupLayout->setContentsMargins(8, 8, 8, 8);
    m_popupLayout->setSpacing(12);
    m_popupLayout->addWidget(m_statusLabel, 0, Qt::AlignCenter);
    m_popupLayout->addStretch();

    m_statusLabel->setAlignment(Qt::AlignCenter);
    m_statusLabel->setWordWrap(true);

    applyStyle();
    connect(&ThemeManager::instance(), &ThemeManager::themeChanged,
            this, &SearchBoxWidget::applyStyle);
}

void SearchBoxWidget::setApiClient(HermindApiClient *apiClient)
{
    m_apiClient = apiClient;
}

void SearchBoxWidget::setPlaceholderText(const QString &text)
{
    m_input->setPlaceholderText(text);
}

QString SearchBoxWidget::placeholderText() const
{
    return m_input->placeholderText();
}

SearchInput *SearchBoxWidget::input() const
{
    return m_input;
}

void SearchBoxWidget::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    positionPopup();
}

void SearchBoxWidget::moveEvent(QMoveEvent *event)
{
    QWidget::moveEvent(event);
    positionPopup();
}

bool SearchBoxWidget::eventFilter(QObject *watched, QEvent *event)
{
    if (watched == m_input) {
        if (event->type() == QEvent::FocusOut) {
            // Popup is a window, so focus out happens when user clicks popup.
            // Delay hiding to allow click events on popup to be processed.
            QTimer::singleShot(200, this, [this]() {
                if (!m_popup->underMouse())
                    hidePopup();
            });
        } else if (event->type() == QEvent::KeyPress) {
            QKeyEvent *keyEvent = static_cast<QKeyEvent *>(event);
            if (keyEvent->key() == Qt::Key_Escape) {
                hidePopup();
                return true;
            }
        }
    }
    return QWidget::eventFilter(watched, event);
}

void SearchBoxWidget::keyPressEvent(QKeyEvent *event)
{
    if (event->key() == Qt::Key_Escape) {
        hidePopup();
        return;
    }
    QWidget::keyPressEvent(event);
}

void SearchBoxWidget::onTextChanged(const QString &text)
{
    if (text.trimmed().length() < kMinSearchTermLength) {
        m_debounceTimer->stop();
        hidePopup();
        return;
    }
    m_debounceTimer->start(kDebounceMs);
}

void SearchBoxWidget::onSearchFinished()
{
    const QString term = m_input->text().trimmed();
    if (term.length() < kMinSearchTermLength) {
        hidePopup();
        return;
    }

    if (!m_apiClient) {
        hidePopup();
        return;
    }

    setLoading(true);
    showPopup();

    QPointer<SearchBoxWidget> guard(this);
    m_apiClient->searchWorkspaceOrThread(term,
        [guard, this](const HermindApiClient::SearchResults &results, const ApiError &error) {
            if (!guard)
                return;
            onResultsReceived(results, error);
        });
}

void SearchBoxWidget::onResultsReceived(const HermindApiClient::SearchResults &results,
                                        const ApiError &error)
{
    setLoading(false);
    if (!error.isEmpty()) {
        m_statusLabel->setText(tr("Search failed: %1").arg(error.message()));
        m_statusLabel->setVisible(true);
        clearResults();
        showPopup();
        return;
    }

    clearResults();

    if (results.workspaces.isEmpty() && results.threads.isEmpty()) {
        m_statusLabel->setText(tr("No results found for \"%1\"").arg(m_input->text().trimmed()));
        m_statusLabel->setVisible(true);
        showPopup();
        return;
    }

    m_statusLabel->setVisible(false);

    if (!results.workspaces.isEmpty()) {
        addCategoryTitle(tr("WORKSPACES"));
        for (const auto &ws : results.workspaces)
            addResultItem(ws.name, QString(), ws.slug, QString());
    }

    if (!results.threads.isEmpty()) {
        addCategoryTitle(tr("THREADS"));
        for (const auto &th : results.threads)
            addResultItem(th.name, th.workspaceName, th.workspaceSlug, th.slug);
    }

    showPopup();
}

void SearchBoxWidget::applyStyle()
{
    const bool dark = ThemeManager::instance().isDarkMode();
    const QString bg = ThemeColors::cardBackground(dark).name();
    const QString border = ThemeColors::border(dark).name();

    m_popup->setStyleSheet(QStringLiteral(
        "QFrame {"
        "  background-color: %1;"
        "  border: 1px solid %2;"
        "  border-radius: 8px;"
        "}"
        "QLabel {"
        "  color: %3;"
        "  font-size: 12px;"
        "}"
    ).arg(bg, border, ThemeColors::textSecondary(dark).name()));
}

void SearchBoxWidget::positionPopup()
{
    if (!m_popup->isVisible())
        return;

    const QPoint globalPos = mapToGlobal(QPoint(0, height()));
    m_popup->setFixedWidth(qMax(width(), 220));
    m_popup->move(globalPos);

    // Keep popup on screen.
    const QScreen *screen = QApplication::screenAt(globalPos);
    if (!screen)
        screen = QApplication::primaryScreen();
    if (screen) {
        const QRect available = screen->availableGeometry();
        const int bottomY = globalPos.y() + m_popup->height();
        if (bottomY > available.bottom()) {
            m_popup->move(globalPos.x(), globalPos.y() - height() - m_popup->height());
        }
    }
}

void SearchBoxWidget::showPopup()
{
    if (m_popup->isVisible()) {
        positionPopup();
        return;
    }
    m_popup->setFixedWidth(qMax(width(), 220));
    m_popup->setMaximumHeight(kPopupMaxHeight);
    positionPopup();
    m_popup->show();
    m_popup->raise();
    m_popup->activateWindow();
}

void SearchBoxWidget::hidePopup()
{
    m_popup->hide();
    m_debounceTimer->stop();
}

void SearchBoxWidget::setLoading(bool loading)
{
    m_loading = loading;
    if (loading) {
        m_statusLabel->setText(tr("Searching for \"%1\"...").arg(m_input->text().trimmed()));
        m_statusLabel->setVisible(true);
        clearResults();
    }
}

void SearchBoxWidget::clearResults()
{
    // Remove all widgets except the status label and stretch.
    while (m_popupLayout->count() > 2) {
        QLayoutItem *item = m_popupLayout->takeAt(1);
        if (item->widget())
            item->widget()->deleteLater();
        delete item;
    }
}

void SearchBoxWidget::addResultItem(const QString &name, const QString &hint,
                                    const QString &workspaceSlug, const QString &threadSlug)
{
    auto *item = new SearchResultItem(name, hint, workspaceSlug, threadSlug, m_popup);
    connect(item, &SearchResultItem::clicked,
            this, [this](const QString &ws, const QString &th) {
                emit resultSelected(ws, th);
                hidePopup();
                m_input->clear();
            });
    // Insert before the stretch at the end.
    m_popupLayout->insertWidget(m_popupLayout->count() - 1, item);
}

void SearchBoxWidget::addCategoryTitle(const QString &title)
{
    auto *label = new QLabel(title, m_popup);
    label->setStyleSheet(QStringLiteral("font-weight: 600; font-size: 11px;"));
    m_popupLayout->insertWidget(m_popupLayout->count() - 1, label);
}

#include "search_box_widget.moc"
