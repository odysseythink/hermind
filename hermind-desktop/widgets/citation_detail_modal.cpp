#include "citation_detail_modal.h"
#include <QVBoxLayout>
#include <QTextBrowser>
#include <QPushButton>

CitationDetailModal::CitationDetailModal(const QJsonObject &source, QWidget *parent)
    : QDialog(parent)
{
    setWindowTitle(QStringLiteral("引用详情"));
    resize(500, 400);

    QVBoxLayout *layout = new QVBoxLayout(this);

    m_browser = new QTextBrowser(this);
    m_browser->setOpenExternalLinks(true);

    QString title = source.value(QStringLiteral("title")).toString(QStringLiteral("Untitled"));
    QString text = source.value(QStringLiteral("text")).toString(
        source.value(QStringLiteral("content")).toString());
    QString url = source.value(QStringLiteral("url")).toString();

    QString html = QStringLiteral("<h2>%1</h2>").arg(title.toHtmlEscaped());
    if (!url.isEmpty())
        html += QStringLiteral("<p><a href=\"%1\">%1</a></p>").arg(url.toHtmlEscaped());
    html += QStringLiteral("<hr/><pre>%1</pre>").arg(text.toHtmlEscaped());

    m_browser->setHtml(html);

    QPushButton *closeBtn = new QPushButton(QStringLiteral("关闭"), this);
    layout->addWidget(m_browser, 1);
    layout->addWidget(closeBtn, 0, Qt::AlignRight);
    connect(closeBtn, &QPushButton::clicked, this, &QDialog::accept);
}
