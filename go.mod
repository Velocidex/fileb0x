module github.com/Velocidex/fileb0x

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/UnnoTed/fileb0x v0.0.0-00010101000000-000000000000
	github.com/bmatcuk/doublestar v1.1.1
	github.com/gizak/termui/v3 v3.1.0
	github.com/karrick/godirwalk v1.7.8
	github.com/labstack/echo v3.2.1+incompatible
	github.com/labstack/gommon v0.3.0 // indirect
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20200117160349-530e935923ad // indirect
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/UnnoTed/fileb0x => ./

go 1.13
