module github.com/langhuihui/gotask-example

go 1.24.0

toolchain go1.24.7

require (
	github.com/gorilla/mux v1.8.1
	github.com/langhuihui/gotask v0.0.0-00010101000000-000000000000
	github.com/ncruces/go-sqlite3/gormlite v0.24.0
	gorm.io/gorm v1.30.0
)

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/ncruces/go-sqlite3 v0.29.0 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace github.com/langhuihui/gotask => ../..
