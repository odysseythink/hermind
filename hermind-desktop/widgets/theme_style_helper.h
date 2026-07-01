#ifndef THEME_STYLE_HELPER_H
#define THEME_STYLE_HELPER_H

#include <QObject>
#include <QWidget>
#include <functional>

class ThemeStyleHelper : public QObject
{
    Q_OBJECT

public:
    using StyleUpdater = std::function<void(QWidget *owner, bool dark)>;

    explicit ThemeStyleHelper(QWidget *owner, StyleUpdater updater, QObject *parent = nullptr);

private slots:
    void onThemeChanged(const QString &theme);

private:
    QWidget *m_owner = nullptr;
    StyleUpdater m_updater;
};

#endif // THEME_STYLE_HELPER_H
