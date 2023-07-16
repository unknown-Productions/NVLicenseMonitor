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
package winService

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"nvLicenseMonitor/internal/nvLicMon"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

type MyService struct{}

var ServiceInstallDir string = `C:\Program Files\NVIDIA Corporation\vGPU Licensing\NVLicenseMonitor\`

func (m *MyService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	jsonData, err := os.ReadFile(filepath.Join(ServiceInstallDir, "config.json"))
	if err != nil {
		log.Fatalf("Failed to read config file: %s", err)
	}
	err = json.Unmarshal(jsonData, &nvLicMon.Config)
	if err != nil {
		log.Fatalf("Failed to deserialize config: %s", err)
	}
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	ticker := time.NewTicker(5 * time.Minute)
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	elog, _ := eventlog.Open("NVLicenseMonitor")
loop:
	for {
		select {
		case <-ticker.C:
			log.Println("Running NVIDIA SMI")
			output := nvLicMon.RunNvidiaSmi()
			log.Println("Checking license")
			isLicensed := nvLicMon.IsLicensed(output)
			if !isLicensed {
				log.Println("Downloading License Token")
				nvLicMon.DownloadLicenseToken()
				log.Println("Restarting service")
				nvLicMon.RestartService(nvLicMon.NVDisplayService)
			}
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			default:
				elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	ticker.Stop()
	changes <- svc.Status{State: svc.StopPending}
	elog.Close()
	return
}

func RunService(name string, isDebug bool, mySvc svc.Handler) {
	var err error
	if isDebug {
		err = svc.Run(name, mySvc)
	} else {
		err = svc.Run(name, mySvc)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func InstallService(name, displayName, nvidiaSmiPath, licensingFilePath, licenseServerUrl string) error {
	// Create service directory if it does not exist
	if _, err := os.Stat(ServiceInstallDir); os.IsNotExist(err) {
		err := os.MkdirAll(ServiceInstallDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %s", err)
		}
	}

	// Get the path of the running executable
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// Get the name of the executable
	_, exeName := filepath.Split(exePath)

	// Define new service executable path
	newExePath := filepath.Join(ServiceInstallDir, exeName)

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err == nil {
		status, err := s.Control(svc.Stop) // Handle both returned values
		if err != nil {
			log.Printf("Could not stop the service: %v", err)
		} else {
			for status.State != svc.Stopped {
				time.Sleep(time.Second * 3)
				status, _ = s.Query()
			}
		}
		err = s.Delete()
		if err != nil {
			s.Close()
			return fmt.Errorf("failed to delete existing service %s: %v", name, err)
		}
		s.Close()
	}

	// Copy the executable to service directory
	err = copyFile(exePath, newExePath)
	if err != nil {
		return fmt.Errorf("failed to copy executable: %s", err)
	}

	// Write config.json to service directory
	configPath := filepath.Join(ServiceInstallDir, "config.json")
	jsonData, err := json.Marshal(nvLicMon.Config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %s", err)
	}
	err = os.WriteFile(configPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %s", err)
	}

	s, err = m.CreateService(name, newExePath, mgr.Config{
		DisplayName: displayName,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return err
	}
	defer s.Close()
	err = s.Start()
	if err != nil {
		return fmt.Errorf("failed to start the service: %s", err)
	}

	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		if strings.Contains(err.Error(), "registry key already exists") {
			// If the event source already exists, we just log a message and continue
			log.Printf("Event source %s already exists", name)
		} else {
			// If the error is caused by something else, we delete the service and return an error
			s.Delete()
			return fmt.Errorf("SetupEventLogSource() failed: %s", err)
		}
	}

	return nil
}

func UninstallService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		log.Printf("Could not stop the service: %v", err)
	} else {
		for status.State != svc.Stopped {
			time.Sleep(time.Second * 3)
			status, _ = s.Query()
		}
	}

	err = s.Delete()
	if err != nil {
		return fmt.Errorf("could not delete service: %v", err)
	}

	// Clean up the installation directory
	err = os.RemoveAll(ServiceInstallDir)
	if err != nil {
		return fmt.Errorf("could not remove service directory: %v", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	sfi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("copyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("copyFile: destination stat failed: %v", err)
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("copyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return nil
		}
	}
	if err = os.Link(src, dst); err == nil {
		return nil
	}
	err = copyFileContents(src, dst)
	return err
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	err = out.Sync()
	return err
}
