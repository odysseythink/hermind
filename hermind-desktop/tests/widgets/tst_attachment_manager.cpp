#include <QtTest>
#include <QSignalSpy>
#include "attachment_manager.h"

class TestAttachmentManager : public QObject
{
    Q_OBJECT
private slots:
    void startsEmpty();
    void addFilesEmitsSignal();
    void removeFileEmitsSignal();
    void clearResets();
};

void TestAttachmentManager::startsEmpty()
{
    AttachmentManager mgr;
    QCOMPARE(mgr.count(), 0);
    QVERIFY(mgr.filePaths().isEmpty());
}

void TestAttachmentManager::addFilesEmitsSignal()
{
    AttachmentManager mgr;
    QSignalSpy spy(&mgr, &AttachmentManager::attachmentsChanged);

    mgr.addFiles({ QStringLiteral("/tmp/a.txt"), QStringLiteral("/tmp/b.jpg") });

    QCOMPARE(spy.count(), 1);
    QCOMPARE(mgr.count(), 2);
    QStringList paths = spy.at(0).at(0).toStringList();
    QVERIFY(paths.contains(QStringLiteral("/tmp/a.txt")));
    QVERIFY(paths.contains(QStringLiteral("/tmp/b.jpg")));
}

void TestAttachmentManager::removeFileEmitsSignal()
{
    AttachmentManager mgr;
    mgr.addFiles({ QStringLiteral("/tmp/a.txt"), QStringLiteral("/tmp/b.jpg") });

    QSignalSpy spy(&mgr, &AttachmentManager::attachmentsChanged);
    mgr.removeFile(QStringLiteral("/tmp/a.txt"));

    QCOMPARE(spy.count(), 1);
    QCOMPARE(mgr.count(), 1);
    QVERIFY(!mgr.filePaths().contains(QStringLiteral("/tmp/a.txt")));
}

void TestAttachmentManager::clearResets()
{
    AttachmentManager mgr;
    mgr.addFiles({ QStringLiteral("/tmp/a.txt") });
    mgr.clear();
    QCOMPARE(mgr.count(), 0);
}

QTEST_MAIN(TestAttachmentManager)
#include "tst_attachment_manager.moc"
