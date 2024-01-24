# UCCNC Lightburn Processor

Converts Lightburn LinuxCNC device profile `*.nc` programs to UCCNC `*_UCCNC.nc` programs.

## Build

`CGO_ENABLED=0 go build -v -o uccnc_lightburn_processor uccnc_lightburn_processor.go`

## Usage

`./uccnc_lightburn_processor` inside your folder that contains Lightburn LinuxCNC device profile exported `*.nc` files.

