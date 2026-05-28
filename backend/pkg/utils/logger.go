package utils

import "github.com/odysseythink/mlog"

func InitLogger(logDir string) {
	mlog.SetEncoder(mlog.NewJSONEncoder())
	mlog.SetLogDir(logDir)
}

func SyncLogger() {
	mlog.Flush()
}
