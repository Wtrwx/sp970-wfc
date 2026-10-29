module github.com/Wtrwx/sp970-wfc

go 1.26.4

require (
	github.com/damonto/euicc-go v1.1.3-0.20260628013808-8d873a2dfc98
	github.com/iniwex5/vowifi-go v0.0.0
	github.com/sagernet/sing v0.6.10
	github.com/sagernet/sing-shadowsocks v0.2.8
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/damonto/uicc-go v0.0.0-20260622025011-5cf36d6c8626 // indirect
	github.com/klauspost/cpuid/v2 v2.0.12 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)

replace github.com/iniwex5/vowifi-go => ./third_party/vowifi-go.v112
