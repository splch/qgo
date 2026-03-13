module github.com/splch/qgo/backend/google

go 1.24

require (
	github.com/splch/qgo v0.0.0
	golang.org/x/oauth2 v0.28.0
)

require (
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace github.com/splch/qgo => ../../
