#ifndef THEMEMANAGER_H
#define THEMEMANAGER_H

#include <QObject>
#include <QString>

class ThemeManager : public QObject
{
    Q_OBJECT
public:
    enum Theme { Dark, Light };
    Q_ENUM(Theme)

    explicit ThemeManager(QObject *parent = nullptr);
    Theme currentTheme() const;
    void setTheme(Theme theme);
    void loadTheme(Theme theme);

signals:
    void themeChanged(Theme theme);

private:
    Theme m_currentTheme;
};

#endif
