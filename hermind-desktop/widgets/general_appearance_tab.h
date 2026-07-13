#ifndef GENERAL_APPEARANCE_TAB_H
#define GENERAL_APPEARANCE_TAB_H

#include <QWidget>

#include "api_response.h"
#include "hermind_workspace.h"

class HermindApiClient;
class QLineEdit;
class SuggestedMessagesEditor;
class QPushButton;

class GeneralAppearanceTab : public QWidget
{
    Q_OBJECT

public:
    explicit GeneralAppearanceTab(HermindApiClient *apiClient,
                                  QWidget *parent = nullptr);

    void setWorkspaceSlug(const QString &slug);

signals:
    void workspaceUpdated(const HermindWorkspace &workspace);
    void workspaceDeleted();

private slots:
    void onWorkspaceLoaded(const HermindWorkspace &workspace,
                           const QString &message,
                           const ApiError &error);
    void onSuggestedMessagesLoaded(const QStringList &messages,
                                   const ApiError &error);
    void onWorkspaceUpdated(const HermindWorkspace &workspace,
                            const QString &message,
                            const ApiError &error);
    void onDeleteFinished(bool success, const ApiError &error);
    void onUpdateNameClicked();
    void onSaveSuggestionsClicked();
    void onDeleteClicked();
    void applyStyle();

private:
    void buildUi();
    void setLoading(bool loading);

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    HermindWorkspace m_workspace;

    QLineEdit *m_nameEdit = nullptr;
    QPushButton *m_updateNameButton = nullptr;
    SuggestedMessagesEditor *m_suggestedEditor = nullptr;
    QPushButton *m_deleteButton = nullptr;
};

#endif // GENERAL_APPEARANCE_TAB_H
