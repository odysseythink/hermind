#!/bin/sh
PATH=/E/Qt-install/6.10.3/llvm-mingw_64/bin:$PATH
export PATH
QT_PLUGIN_PATH=/E/Qt-install/6.10.3/llvm-mingw_64/plugins${QT_PLUGIN_PATH:+:$QT_PLUGIN_PATH}
export QT_PLUGIN_PATH
exec "$@"
