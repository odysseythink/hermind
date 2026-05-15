#ifndef HERMINDBACKENDPROCESS_H
#define HERMINDBACKENDPROCESS_H

#include <QObject>
#include <QProcess>
#include <QHostAddress>

class HermindProcess : public QObject
{
    Q_OBJECT
public:
    explicit HermindProcess(QObject *parent = nullptr);
    void start();
    void shutdown();
    bool isRunning() const;

signals:
    void backendReady(const QHostAddress &address, int port);
    void backendError(const QString &message);
    void backendFinished();

private slots:
    void onReadyReadStandardOutput();
    void onErrorOccurred(QProcess::ProcessError error);
    void onFinished(int exitCode, QProcess::ExitStatus status);

private:
    QProcess *m_process;
    int m_port;
};

#endif
