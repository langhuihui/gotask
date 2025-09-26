module github.com/langhuihui/gotask-example

go 1.23

require (
	github.com/gorilla/mux v1.8.1
	github.com/langhuihui/gotask v0.0.0-00010101000000-000000000000
	github.com/mattn/go-sqlite3 v1.14.19
)

replace github.com/langhuihui/gotask => ../..
