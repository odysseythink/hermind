#include "HermindProcess.h"
#include <QCoreApplication>
#include <QRegularExpression>
#include <QDebug>
#include <QFileInfo>

#ifdef Q_OS_WIN
#include <windows.h>
#endif

HermindProcess::HermindProcess(QObject *parent)
    : QObject(parent), m_process(new QProcess(this)), m_port(0)
{
    connect(m_process, &QProcess::readyReadStandardOutput,
            this, &HermindProcess::onReadyReadStandardOutput);
    connect(m_process, QOverload<QProcess::ProcessError>::of(&QProcess::errorOccurred),
            this, &HermindProcess::onErrorOccurred);
    connect(m_process, QOverload<int, QProcess::ExitStatus>::of(&QProcess::finished),
            this, &HermindProcess::onFinished);
}

void HermindProcess::start()
{
    QStringList candidates;
    QString appDir = QCoreApplication::applicationDirPath();
    candidates << appDir + "/hermind";
    candidates << appDir + "/hermind.exe";
    candidates << appDir + "/../hermind";
    candidates << appDir + "/../hermind.exe";
    candidates << appDir + "/../bin/hermind";
    candidates << appDir + "/../bin/hermind.exe";
    candidates << appDir + "/../../bin/hermind";
    candidates << appDir + "/../../bin/hermind.exe";

    QString goBinary;
    for (const QString &c : candidates) {
        QString canonical = QFileInfo(c).canonicalFilePath();
        if (!canonical.isEmpty() && QFileInfo::exists(canonical)) {
            goBinary = canonical;
            break;
        }
    }

    if (goBinary.isEmpty()) {
        emit backendError("Backend binary not found. Searched: " + candidates.join(", "));
        return;
    }

    m_process->setProgram(goBinary);
    m_process->setArguments(QStringList() << "desktop");

#ifdef Q_OS_WIN
    m_process->setCreateProcessArgumentsModifier(
        [](QProcess::CreateProcessArguments *args) {
            args->flags |= CREATE_NO_WINDOW;
        });
#endif

    m_process->start();
}

void HermindProcess::shutdown()
{
    if (m_process->state() != QProcess::Running)
        return;

    m_process->terminate();
    if (!m_process->waitForFinished(5000)) {
        m_process->kill();
    }
}

bool HermindProcess::isRunning() const
{
    return m_process->state() == QProcess::Running;
}

void HermindProcess::onReadyReadStandardOutput()
{
    QString output = m_process->readAllStandardOutput();
    QRegularExpression re("HERMIND_READY (\\d+)");
    QRegularExpressionMatch match = re.match(output);
    if (match.hasMatch()) {
        m_port = match.captured(1).toInt();
        emit backendReady(QHostAddress::LocalHost, m_port);
    }
}

void HermindProcess::onErrorOccurred(QProcess::ProcessError error)
{
    emit backendError(m_process->errorString());
}

void HermindProcess::onFinished(int exitCode, QProcess::ExitStatus status)
{
    emit backendFinished();
}
