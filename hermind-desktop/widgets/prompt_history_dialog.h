#ifndef PROMPT_HISTORY_DIALOG_H
#define PROMPT_HISTORY_DIALOG_H

#include <QDialog>
#include <QJsonArray>
#include <QString>

class HermindApiClient;
class QVBoxLayout;

class PromptHistoryDialog : public QDialog
{
    Q_OBJECT

public:
    PromptHistoryDialog(HermindApiClient *apiClient,
                        const QString &workspaceSlug,
                        QWidget *parent = nullptr);

signals:
    void promptSelected(const QString &prompt);

private:
    void loadHistory();
    void renderHistory(const QJsonArray &history);
    void deleteItem(int id);
    void clearAll();

    HermindApiClient *m_apiClient = nullptr;
    QString m_workspaceSlug;
    QVBoxLayout *m_listLayout = nullptr;
};

#endif // PROMPT_HISTORY_DIALOG_H
