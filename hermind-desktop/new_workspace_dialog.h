#ifndef NEW_WORKSPACE_DIALOG_H
#define NEW_WORKSPACE_DIALOG_H

#include <QDialog>
#include <QPointer>

#include "hermind_workspace.h"

class HermindApiClient;
class QLineEdit;
class QLabel;
class QPushButton;
class QDialogButtonBox;
class ThemeStyleHelper;

class NewWorkspaceDialog : public QDialog
{
    Q_OBJECT

public:
    explicit NewWorkspaceDialog(QWidget *parent = nullptr);

    void setApiClient(HermindApiClient *apiClient);

signals:
    void workspaceCreated(const HermindWorkspace &workspace);

private slots:
    void onSaveClicked();
    void onTextChanged();
    void applyStyle(bool dark);

private:
    void setLoading(bool loading);
    void setError(const QString &message);
    void resetForm();

    HermindApiClient *m_apiClient = nullptr;
    QLineEdit *m_nameEdit = nullptr;
    QLabel *m_errorLabel = nullptr;
    QPushButton *m_saveButton = nullptr;
    QDialogButtonBox *m_buttonBox = nullptr;
    ThemeStyleHelper *m_styleHelper = nullptr;
    bool m_loading = false;
};

#endif // NEW_WORKSPACE_DIALOG_H
