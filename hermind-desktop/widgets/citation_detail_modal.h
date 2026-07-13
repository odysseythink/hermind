#ifndef CITATION_DETAIL_MODAL_H
#define CITATION_DETAIL_MODAL_H

#include <QDialog>
#include <QJsonObject>

class QTextBrowser;

class CitationDetailModal : public QDialog
{
    Q_OBJECT
public:
    explicit CitationDetailModal(const QJsonObject &source, QWidget *parent = nullptr);

private:
    QTextBrowser *m_browser = nullptr;
};

#endif // CITATION_DETAIL_MODAL_H
