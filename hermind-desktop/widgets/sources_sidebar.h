#ifndef SOURCES_SIDEBAR_H
#define SOURCES_SIDEBAR_H

#include <QWidget>
#include <QJsonArray>

class QVBoxLayout;
class QLabel;
class QPushButton;
class QScrollArea;

class SourcesSidebar : public QWidget
{
    Q_OBJECT
public:
    explicit SourcesSidebar(QWidget *parent = nullptr);

    void setSources(const QJsonArray &sources);
    void clear();

    bool isOpen() const;

signals:
    void closeRequested();

public slots:
    void open();
    void close();

private:
    void applyTheme();
    void rebuild();
    QJsonArray combineLikeSources(const QJsonArray &sources) const;

    QJsonArray m_sources;
    bool m_open = false;

    QScrollArea *m_scroll = nullptr;
    QWidget *m_container = nullptr;
    QVBoxLayout *m_listLayout = nullptr;
    QLabel *m_header = nullptr;
    QPushButton *m_closeBtn = nullptr;
};

#endif // SOURCES_SIDEBAR_H
