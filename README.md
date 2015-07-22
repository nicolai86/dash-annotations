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


## TODO

- forgotten password handling (request/ reset)
