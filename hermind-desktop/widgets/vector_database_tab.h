#ifndef VECTOR_DATABASE_TAB_H
#define VECTOR_DATABASE_TAB_H

#include <QWidget>

#include "api_response.h"
#include "hermind_workspace.h"

class HermindApiClient;
class QComboBox;
class QLabel;
class QPushButton;
class QSpinBox;

class VectorDatabaseTab : public QWidget
{
    Q_OBJECT

public:
    explicit VectorDatabaseTab(HermindApiClient *apiClient,
                               QWidget *parent = nullptr);

    void setWorkspaceSlug(const QString &slug);

signals:
    void workspaceUpdated(const HermindWorkspace &workspace);
    void hasChangesChanged(bool hasChanges);

private slots:
    void onWorkspaceLoaded(const HermindWorkspace &workspace,
                           const QString &message,
                           const ApiError &error);
    void onSystemKeysLoaded(const QJsonObject &settings,
                            const ApiError &error);
    void onVectorCountLoaded(int count,
                             const ApiError &error);
    void onWorkspaceUpdated(const HermindWorkspace &workspace,
                            const QString &message,
                            const ApiError &error);
    void onWipeFinished(bool success,
                        const ApiError &error);

    void onFieldEdited();
    void onSaveClicked();
    void onResetClicked();
    void applyStyle();

private:
    void buildUi();
    void setLoading(bool loading);
    void updateSearchModeVisibility();
    void updateHasChanges();
    void loadVectorCount();
    QJsonObject collectFields() const;

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    HermindWorkspace m_workspace;
    QString m_vectorDb;
    bool m_hasChanges = false;

    QLabel *m_identifierLabel = nullptr;
    QLabel *m_vectorCountLabel = nullptr;
    QComboBox *m_searchModeCombo = nullptr;
    QSpinBox *m_topNSpin = nullptr;
    QComboBox *m_thresholdCombo = nullptr;
    QPushButton *m_resetButton = nullptr;
    QPushButton *m_saveButton = nullptr;
};

#endif // VECTOR_DATABASE_TAB_H
