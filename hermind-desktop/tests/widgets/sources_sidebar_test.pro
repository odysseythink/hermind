QT += widgets testlib
CONFIG += qt warn_on depend_includepath testcase c++17

TEMPLATE = app
TARGET = tst_sources_sidebar

INCLUDEPATH += $$PWD/../../widgets $$PWD/../../models $$PWD/../..

SOURCES += \
    tst_sources_sidebar.cpp \
    ../../widgets/sources_sidebar.cpp \
    ../../widgets/source_item.cpp \
    ../../widgets/citation_detail_modal.cpp \
    ../../widgets/theme_colors.cpp \
    ../../settings_store.cpp \
    ../../theme_manager.cpp

HEADERS += \
    ../../widgets/sources_sidebar.h \
    ../../widgets/source_item.h \
    ../../widgets/citation_detail_modal.h \
    ../../widgets/theme_colors.h \
    ../../settings_store.h \
    ../../theme_manager.h
