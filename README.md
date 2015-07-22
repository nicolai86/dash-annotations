# Dash Annotations

a Dash 3 compatible annotations server written in go

## Usage

Assuming you're trying to get the annotations backend up and running for the first time:

- create a mysql db, I'll call mine `dash3`

- build the project:

      ```$ gb build```

- run the migrations:

    ```$ ./bin/migrate -datasource="root:@/dash3?parseTime=true"```

- start the api:

    ```$ ./bin/server --datasource="root@/dash3"```

- lastly, instruct dash to talk to your API instead of the public api:

    ```defaults write com.kapeli.dashdoc AnnotationsCustomServer "http://localhost:8000"```

## Running on OS X

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
    <string>--datasource="root@/dash3"</string>
    <string>--session.secret=1234123412341234</string>
    <string>--listen=127.0.0.1:54111</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>/usr/local/var</string>
</dict>
</plist>
EOF
```

## TODO

- forgotten password handling (request/ reset)
