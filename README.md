# Dash Annotations

[![wercker status](https://app.wercker.com/status/bff7a133f740428731b816db04a53265/m "wercker status")](https://app.wercker.com/project/bykey/bff7a133f740428731b816db04a53265)

a Dash 3 compatible annotations server written in go

## features

- tiny footprint & minimal dependencies
- supports mysql & sqlite3
- full offline support (annotation rendering does not load bootstrap from cdn)

## Usage

Assuming you're trying to get the annotations backend up and running for the first time:

- create a mysql db, I'll call mine `dash3`

- build the project:

      ```$ gb build```

- run the migrations:

    ```$ ./bin/migrate -datasource="root:@/dash3?parseTime=true" -driver=mysql```

- start the api:

    ```$ ./bin/server -datasource="root@/dash3" -driver=mysql```

- lastly, instruct dash to talk to your API instead of the public api:

    ```defaults write com.kapeli.dashdoc AnnotationsCustomServer "http://localhost:8000"```

## Running on OS X

The below file will setup a `launchd` configuration and launch the API using sqlite3 as storage engine - for a minimal dependency footprint.

```
sudo mkdir -p /var/run/rra.kapeli/
sudo chown $(whoami) /var/run/rra.kapeli/
$GOPATH/src/github.com/nicolai86/dash-annotations/bin/migrate --driver=sqlite3 --datasource="$HOME/Library/dash-annotations/dash.sqlite3"
```

``` xml
cat <<EOF > $HOME/Library/LaunchAgents/rra.kapeli.annotations.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>KeepAlive</key>
  <true/>
  <key>Label</key>
  <string>rra.kapeli.annotations</string>
  <key>ProgramArguments</key>
  <array>
    <string>$GOPATH/src/github.com/nicolai86/dash-annotations/bin/server</string>
    <string>--driver=sqlite3</string>
    <string>--datasource=$HOME/Library/dash-annotations/dash.sqlite3</string>
    <string>--session.secret=1234123412341234</string>
    <string>--listen=127.0.0.1:54111</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>/var/run/rra.kapeli</string>
  <key>StandardOutPath</key>
  <string>/var/run/rra.kapeli/stdout</string>
  <key>StandardErrorPath</key>
  <string>/var/run/rra.kapeli/stderr</string>
</dict>
</plist>
EOF
```

## TODO

- forgotten password handling (request/ reset)
