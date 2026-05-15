#ifndef EMPTYSTATEWIDGET_H
#define EMPTYSTATEWIDGET_H

#include <QWidget>
#include <QStringList>

class HermindClient;
class QVBoxLayout;
class QHBoxLayout;

class EmptyStateWidget : public QWidget
{
    Q_OBJECT
public:
    explicit EmptyStateWidget(QWidget *parent = nullptr);
    void setClient(HermindClient *client);

signals:
    void suggestionClicked(const QString &text);

private slots:
    void refreshSuggestions();

private:
    void setupUI();
    void buildSuggestionButtons(const QStringList &suggestions);

    HermindClient *m_client;
    QVBoxLayout *m_mainLayout;
    QHBoxLayout *m_suggestionLayout;
};

#endif
