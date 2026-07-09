#ifndef SEARCH_BOX_WIDGET_H
#define SEARCH_BOX_WIDGET_H

#include <QWidget>
#include <QTimer>
#include <QPointer>

#include "hermind_api_client.h"

class SearchInput;
class QFrame;
class QVBoxLayout;
class QLabel;
class HermindApiClient;

class SearchBoxWidget : public QWidget
{
    Q_OBJECT

public:
    explicit SearchBoxWidget(QWidget *parent = nullptr);

    void setApiClient(HermindApiClient *apiClient);

    void setPlaceholderText(const QString &text);
    QString placeholderText() const;

    SearchInput *input() const;

signals:
    void resultSelected(const QString &workspaceSlug, const QString &threadSlug);

protected:
    void resizeEvent(QResizeEvent *event) override;
    void moveEvent(QMoveEvent *event) override;
    bool eventFilter(QObject *watched, QEvent *event) override;
    void keyPressEvent(QKeyEvent *event) override;

private slots:
    void onTextChanged(const QString &text);
    void onSearchFinished();
    void onResultsReceived(const HermindApiClient::SearchResults &results,
                           const ApiError &error);
    void applyStyle();

private:
    void positionPopup();
    void showPopup();
    void hidePopup();
    void setLoading(bool loading);
    void clearResults();
    void addResultItem(const QString &name, const QString &hint,
                       const QString &workspaceSlug, const QString &threadSlug);
    void addCategoryTitle(const QString &title);

    HermindApiClient *m_apiClient = nullptr;
    SearchInput *m_input = nullptr;
    QFrame *m_popup = nullptr;
    QVBoxLayout *m_popupLayout = nullptr;
    QLabel *m_statusLabel = nullptr;
    QTimer *m_debounceTimer = nullptr;
    bool m_loading = false;
};

#endif // SEARCH_BOX_WIDGET_H
