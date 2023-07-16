# NVLicenseMonitor

NVLicenseMonitor is a service that monitors NVIDIA vGPU licenses and downloads a new license token when needed. This service can be run as a standalone executable or installed as a Windows service. It periodically checks the status of NVIDIA vGPU licenses using the NVIDIA System Management Interface (nvidia-smi). If a license is not found or if the license is expired, the service downloads a new license token from the specified license server.

## Prerequisites

- Go installed on your machine, if you wish to build from source.
- NVIDIA vGPU driver installed on your machine.
- Administrative privileges to install/uninstall the service.

## Installation

Download from releases or clone this repository and build with Go:

    go build

## Usage

You can run NVLicenseMonitor as a standalone executable or install it as a Windows service.

### Standalone Executable

    NVLicenseMonitor.exe -u https://example.com/genClientToken

### Windows Service

To install as a Windows service:

    NVLicenseMonitor.exe install -u https://example.com/genClientToken

To uninstall the service:

    NVLicenseMonitor.exe uninstall

### Parameters

The following parameters can be used with the NVLicenseMonitor:

- `--LicenseServerUrl` - URL of the license server. This is a required parameter.
- `--NvidiaSmiPath` - Path to the NVIDIA SMI executable. Defaults to `C:\Windows\System32\nvidia-smi.exe`.
- `--LicensingFilePath` - Path to the licensing file. Defaults to `C:\Program Files\NVIDIA Corporation\vGPU Licensing\ClientConfigToken\`.
- `--IgnoreSSL` - Ignore SSL for HTTPS connections. Defaults to `false`.

## License

This project is licensed under the GPLv3 License - see the [LICENSE](LICENSE) file for details.
