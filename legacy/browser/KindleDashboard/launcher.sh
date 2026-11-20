#!/bin/sh

app=/mnt/us/KindleDashboard
server="$app/dashboard-server"
log="$app/dashboard.log"

if [ ! -r "$server" ] || [ ! -r "$app/web/index.html" ]; then
    echo "Copy the entire KindleDashboard folder beside documents."
    exit 1
fi

case "${1:-start}" in
    stop)
        if "$server" -check >/dev/null 2>&1; then
            "$server" -stop || exit 1
        fi
        echo "Personal Dashboard server stopped."
        exit 0
        ;;
    start) ;;
    *) echo "Usage: launcher.sh start|stop"; exit 2 ;;
esac

if ! "$server" -check >/dev/null 2>&1; then
    (
        trap '' HUP
        exec "$server" -kindle -dir "$app/web"
    ) </dev/null >"$log" 2>&1 &
    ready=no
    for attempt in 1 2 3 4 5; do
        if "$server" -check >/dev/null 2>&1; then
            ready=yes
            break
        fi
        sleep 1
    done
    if [ "$ready" != yes ]; then
        echo "Dashboard could not start. Check KindleDashboard/dashboard.log."
        exit 1
    fi
fi

echo "Open http://127.0.0.1:8080/ in your Kindle browser."
lipc-set-prop com.lab126.appmgrd start app://com.lab126.browser >>"$log" 2>&1 || :
