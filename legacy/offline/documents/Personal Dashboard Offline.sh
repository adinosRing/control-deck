#!/bin/sh
# Name: Personal Dashboard Offline
# Author: Personal Dashboard
# DontUseFBInk

app=/mnt/us/KindleDashboardNative
log="$app/dashboard.log"
binary="$app/personal-dashboard"

if [ ! -x "$binary" ]; then
    /mnt/us/libkh/bin/fbink -y 4 "Copy KindleDashboardNative beside documents."
    exit 1
fi

if ! "$binary" >"$log" 2>&1; then
    if [ -x /mnt/us/libkh/bin/fbink ]; then
        /mnt/us/libkh/bin/fbink -y 4 -r <"$log"
        sleep 5
    fi
    exit 1
fi
