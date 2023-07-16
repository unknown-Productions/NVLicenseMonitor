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
package license

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const gplv3URL = "https://www.gnu.org/licenses/gpl-3.1.txt"

const CopyrightText = "NVLicenseMonitor  Copyright (C) 2023  unknown.Productions \nThis program comes with ABSOLUTELY NO WARRANTY; for details run `NVLicenseMonitor.exe warranty'.\nThis is free software, and you are welcome to redistribute it under certain conditions; run `NVLicenseMonitor.exe license' for details.\n"
const WarrantyText = "This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details."

func PrintGPLv3Text() {
	// Fetch the GPLv3 license text from the GNU servers
	response, err := http.Get(gplv3URL)
	if err != nil {
		log.Printf("Failed to fetch the GPLv3 license: %v \n To view the license please visit https://www.gnu.org/licenses/gpl-3.0", err)
		return
	}
	defer response.Body.Close()

	// Read the response body
	content, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Failed to read the GPLv3 license response: %v \n To view the license please visit https://www.gnu.org/licenses/gpl-3.0", err)
		return
	}

	// Find the index of the "END OF TERMS AND CONDITIONS" line
	endIndex := strings.Index(string(content), "END OF TERMS AND CONDITIONS")
	if endIndex == -1 {
		log.Println("Failed to locate the end of the GPLv3 license terms \n To view the license please visit https://www.gnu.org/licenses/gpl-3.0")
		return
	}

	// Print the contents of the license up until the "END OF TERMS AND CONDITIONS" line
	fmt.Println(string(content[:endIndex]))
}
