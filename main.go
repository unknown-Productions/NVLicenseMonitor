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
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"nvLicenseMonitor/internal/license"
	"nvLicenseMonitor/internal/nvLicMon"
	"nvLicenseMonitor/internal/winService"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sys/windows/svc"

	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	fmt.Print(license.CopyrightText)
	// Listen for termination signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		log.Printf("Received termination signal: %s. Exiting...", sig)
		os.Exit(1)
	}()

	isIntSess, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to determine if we are running in an interactive session: %v", err)
	}
	if isIntSess {
		logger := &lumberjack.Logger{
			Filename:   filepath.Join(winService.ServiceInstallDir, "nvLicenseMonitor.log"),
			MaxSize:    1,    // max megabytes before log is rotated
			MaxBackups: 1,    // max number of backups to keep
			MaxAge:     28,   // max number of days to retain old log files
			Compress:   true, // compress the backups (using gzip)
		}
		defer logger.Close()
		log.SetOutput(logger)
		winService.RunService("NVLicenseMonitor", false, &winService.MyService{})
		return
	}

	pflag.StringVarP(&nvLicMon.NvidiaSmiPath, "NvidiaSmiPath", "s", `C:\Windows\System32\nvidia-smi.exe`, "Path to the NVIDIA SMI executable")
	pflag.StringVarP(&nvLicMon.LicensingFilePath, "LicensingFilePath", "f", `C:\Program Files\NVIDIA Corporation\vGPU Licensing\ClientConfigToken\`, "Path to the licensing file")
	pflag.BoolVarP(&nvLicMon.IgnoreSSL, "IgnoreSSL", "i", false, "Ignore SSL for HTTPS connections")
	pflag.StringVarP(&nvLicMon.LicenseServerUrl, "LicenseServerUrl", "u", "", "REQUIRED: URL of the license server")

	// Parse the flags.
	pflag.Parse()

	nvLicMon.Config = nvLicMon.ConfigT{
		NvidiaSmiPath:     nvLicMon.NvidiaSmiPath,
		LicensingFilePath: nvLicMon.LicensingFilePath,
		LicenseServerUrl:  nvLicMon.LicenseServerUrl,
		IgnoreSSL:         nvLicMon.IgnoreSSL,
	}
	jsonData, err := json.Marshal(nvLicMon.Config)
	if err != nil {
		log.Fatalf("Failed to serialize config: %s", err)
	}
	err = os.WriteFile("config.json", jsonData, 0644)
	if err != nil {
		log.Fatalf("Failed to write config file: %s", err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(pflag.CommandLine)

	var RootCmd = &cobra.Command{
		Use:   "NVLicenseMonitor",
		Short: "Monitors NVIDIA vGPU licenses",
		Long:  `Monitors NVIDIA vGPU licenses and downloads a new license token when needed.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.MarkFlagRequired("LicenseServerUrl"); err != nil {
				log.Fatalf("Failed to mark LicenseServerUrl as required: %s", err)
			}
			if !isElevated() {
				log.Fatalf("You need to run this command with administrative privileges.")
			}
			nvLicMon.Execute()
		},
	}

	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Install NVLicenseMonitor as a service",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.MarkFlagRequired("LicenseServerUrl"); err != nil {
				log.Fatalf("Failed to mark LicenseServerUrl as required: %s", err)
			}
			if !isElevated() {
				log.Fatalf("You need to run this command with administrative privileges.")
			}
			if nvLicMon.LicenseServerUrl == "" {
				log.Fatalf("LicenseServerUrl is required for installing the service.")
			}
			err := winService.InstallService("NVLicenseMonitor", "NVIDIA License Monitor", nvLicMon.NvidiaSmiPath, nvLicMon.LicensingFilePath, nvLicMon.LicenseServerUrl)
			if err != nil {
				log.Fatalf("Failed to install service: %s", err)
			}
		},
	}

	var uninstallCmd = &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall NVLicenseMonitor service",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Flags().Parse([]string{}); err != nil {
				log.Fatalf("Error parsing flags: %v", err)
			}
			if !isElevated() {
				log.Fatalf("You need to run this command with administrative privileges.")
			}
			err := winService.UninstallService("NVLicenseMonitor")
			if err != nil {
				log.Fatalf("Failed to uninstall service: %s", err)
			}
		},
	}

	var licenseCmd = &cobra.Command{
		Use:   "license",
		Short: "Prints the GPLv3 license",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Flags().Parse([]string{}); err != nil {
				log.Fatalf("Error parsing flags: %v", err)
			}
			license.PrintGPLv3Text()
		},
	}

	RootCmd.AddCommand(installCmd, uninstallCmd, licenseCmd)
	if err := RootCmd.Execute(); err != nil {
		log.Printf("Error executing RootCmd: %s", err)
		os.Exit(1)
	}
}

func isElevated() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		log.Println("Insufficient privileges.")
		return false
	}
	log.Println("Running with necessary privileges.")
	return true
}
