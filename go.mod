module termium

go 1.26.0

require (
	github.com/creack/pty v1.1.24
	github.com/gdamore/tcell/v3 v3.6.0
	github.com/hinshun/vt10x v0.0.0-20220301184237-5011da428d02
	github.com/mattn/go-runewidth v0.0.16
	github.com/mattn/go-sixel v0.0.5
	github.com/rivo/uniseg v0.4.7
	golang.org/x/image v0.20.0
	golang.org/x/sys v0.48.0
	golang.org/x/term v0.46.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/soniakeys/quant v1.0.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
)

replace github.com/mattn/go-sixel => ./third_party/go-sixel
