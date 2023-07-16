// NVLicenseMonitor - A service to monitor NVIDIA vGPU licensing and download a new license token when needed.
// Copyright (C) 2023 unknown.Productions

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
package nvLicMon

import (
	"crypto/tls"
	"encoding/xml"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type ConfigT struct {
	NvidiaSmiPath     string `json:"NvidiaSmiPath"`
	LicensingFilePath string `json:"LicensingFilePath"`
	LicenseServerUrl  string `json:"LicenseServerUrl"`
	IgnoreSSL         bool   `json:"IgnoreSSL"`
}

var Config ConfigT

var (
	NvidiaSmiPath     = `C:\Windows\System32\nvidia-smi.exe`
	LicensingFilePath = `C:\Program Files\NVIDIA Corporation\vGPU Licensing\ClientConfigToken\`
	LicenseServerUrl  string
	IgnoreSSL         bool
)

const NVDisplayService = "NVDisplay.ContainerLocalSystem"

type nvidiaSmiLog struct {
	XMLName xml.Name `xml:"nvidia_smi_log"`
	GPU     struct {
		VGPUSoftwareLicensedProduct struct {
			LicensedProductName string `xml:"licensed_product_name"`
			LicenseStatus       string `xml:"license_status"`
		} `xml:"vgpu_software_licensed_product"`
	} `xml:"gpu"`
}

func Execute() {
	log.Println("Running NVIDIA SMI")
	output := RunNvidiaSmi()
	log.Println("Checking license")
	isLicensed := IsLicensed(output)

	if !isLicensed {
		log.Println("Downloading License Token")
		DownloadLicenseToken()
		log.Println("Restarting service")
		RestartService(NVDisplayService)
	}
}

func RunNvidiaSmi() string {
	cmd := exec.Command(NvidiaSmiPath, "-q", "-x")
	output, _ := cmd.Output()
	return string(output)
}

func IsLicensed(output string) bool {
	var log nvidiaSmiLog
	xml.Unmarshal([]byte(output), &log)
	licenseStatus := log.GPU.VGPUSoftwareLicensedProduct.LicenseStatus

	// Check if the license status includes an expiry date
	regex := regexp.MustCompile(`Licensed \(Expiry: [\d-: ]+ GMT\)`)
	return regex.MatchString(licenseStatus)
}

func DownloadLicenseToken() {
	var resp *http.Response
	var err error
	if IgnoreSSL {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(LicenseServerUrl)
	} else {
		resp, err = http.Get(LicenseServerUrl)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Get the filename from the Content-Disposition header
	_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err != nil {
		log.Fatal(err)
	}

	filename := params["filename"]
	// Replace any colons with underscores
	filename = strings.ReplaceAll(filename, ":", "_")

	filePath := filepath.Join(LicensingFilePath, filename)
	outFile, err := os.Create(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	oldFiles, _ := filepath.Glob(filepath.Join(LicensingFilePath, "*.tok"))
	for _, oldFile := range oldFiles {
		if oldFile != filePath {
			os.Remove(oldFile)
		}
	}
}

func RestartService(serviceName string) {
	exec.Command("net", "stop", serviceName).Run()
	exec.Command("net", "start", serviceName).Run()
}
