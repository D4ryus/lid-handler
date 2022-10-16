# Lid-Handler

A Lid-Handler daemon that listens for dbus signals from UPower and sets the screen brightness to zero when the Lid is closed and restores it when opened.

## Build

```
go get lid-handler
go build
```

## As systemd user service

Save (or link) `lid-landler.service` to `~/.config/systemd/user/` and make sure the `lid-handler` executable can be found, followed by:

```
systemctl --user daemon-reload
systemctl --user enable lid-handler
systemctl --user start lid-handler
```
