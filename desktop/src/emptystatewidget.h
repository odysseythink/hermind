#ifndef EMPTYSTATEWIDGET_H
#define EMPTYSTATEWIDGET_H

#include <QWidget>

class EmptyStateWidget : public QWidget
{
    Q_OBJECT
public:
    explicit EmptyStateWidget(QWidget *parent = nullptr);

signals:
    void suggestionClicked(const QString &text);

private:
    void setupUI();
};

#endif
