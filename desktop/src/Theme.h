#ifndef THEME_H
#define THEME_H

#include <QObject>
#include <QColor>

class Theme : public QObject
{
    Q_OBJECT
    Q_PROPERTY(bool isDark READ isDark WRITE setIsDark NOTIFY isDarkChanged)
    Q_PROPERTY(QColor bg READ bg NOTIFY isDarkChanged)
    Q_PROPERTY(QColor surface READ surface NOTIFY isDarkChanged)
    Q_PROPERTY(QColor surfaceHover READ surfaceHover NOTIFY isDarkChanged)
    Q_PROPERTY(QColor border READ border NOTIFY isDarkChanged)
    Q_PROPERTY(QColor textPrimary READ textPrimary NOTIFY isDarkChanged)
    Q_PROPERTY(QColor textSecondary READ textSecondary NOTIFY isDarkChanged)
    Q_PROPERTY(QColor accent READ accent CONSTANT)
    Q_PROPERTY(QColor accentHover READ accentHover NOTIFY isDarkChanged)
    Q_PROPERTY(QColor success READ success NOTIFY isDarkChanged)
    Q_PROPERTY(QColor error READ error NOTIFY isDarkChanged)
    Q_PROPERTY(QColor warning READ warning NOTIFY isDarkChanged)
    Q_PROPERTY(QColor codeBg READ codeBg NOTIFY isDarkChanged)

public:
    explicit Theme(QObject *parent = nullptr);

    bool isDark() const { return m_isDark; }
    void setIsDark(bool dark);

    QColor bg() const { return m_isDark ? QColor("#0a0b0d") : QColor("#fafaf7"); }
    QColor surface() const { return m_isDark ? QColor("#14161a") : QColor("#f0ede4"); }
    QColor surfaceHover() const { return m_isDark ? QColor("#1d2027") : QColor("#e8e4d6"); }
    QColor border() const { return m_isDark ? QColor("#2a2e36") : QColor("#d9d4c8"); }
    QColor textPrimary() const { return m_isDark ? QColor("#e8e6e3") : QColor("#1a1817"); }
    QColor textSecondary() const { return m_isDark ? QColor("#8a8680") : QColor("#5c584f"); }
    QColor accent() const { return QColor("#FFB800"); }
    QColor accentHover() const { return m_isDark ? QColor("#FF8A00") : QColor("#E89F00"); }
    QColor success() const { return m_isDark ? QColor("#7ee787") : QColor("#2f7d32"); }
    QColor error() const { return m_isDark ? QColor("#ff6b6b") : QColor("#c9302c"); }
    QColor warning() const { return m_isDark ? QColor("#d29922") : QColor("#b37b04"); }
    QColor codeBg() const { return QColor("#1d2027"); }

signals:
    void isDarkChanged();

private:
    bool m_isDark = true;
};

#endif // THEME_H
