# cmd/migrate

bootstrap the dash annotations database

In reality you'll just have to run this once, e.g. like this:

```
$ ./bin/migrate -datasource="root:@/dash3-test?parseTime=true" -driver=mysql
// or, for sqlite3
$ ./bin/migrate -datasource="dash.sqlite3" -driver=sqlite3
```
