// Mgmt
// Copyright (C) James Shubin and the project contributors
// Written by James Shubin <james@shubin.ca> and the project contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Additional permission under GNU GPL version 3 section 7
//
// If you modify this program, or any covered work, by linking or combining it
// with embedded mcl code and modules (and that the embedded mcl code and
// modules which link with this program, contain a copy of their source code in
// the authoritative form) containing parts covered by the terms of any other
// license, the licensors of this program grant you additional permission to
// convey the resulting work. Furthermore, the licensors of this program grant
// the original author, James Shubin, additional permission to update this
// additional permission if he deems it necessary to achieve the goals of this
// additional permission.

package coreos

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/purpleidea/mgmt/lang/funcs/simple"
	"github.com/purpleidea/mgmt/lang/types"
)

var virtualizationVendorMap = map[string]string{
	"KVM":                    "KVM",
	"OpenStack":              "KVM",
	"KubeVirt":               "KVM",
	"Amazon EC2":             "Amazon",
	"QEMU":                   "QEMU",
	"VMware":                 "VMware",
	"VMW":                    "VMware",
	"innotek GmbH":           "Oracle",
	"VirtualBox":             "Oracle",
	"Oracle Corporation":     "Oracle",
	"Xen":                    "Xen",
	"Bochs":                  "Bochs",
	"Parallels":              "Parallels",
	"BHYVE":                  "BHYVE",
	"Hyper-V":                "Microsoft",
	"Apple Virtualization":   "Apple",
	"Google Computer Engine": "Google",
}

var dmiFilesSlice = []string{
	"/sys/class/dmi/id/product_name",
	"/sys/class/dmi/id/sys_vendor",
	"/sys/class/dmi/id/board_vendor",
	"/sys/class/dmi/id/bios_vendor",
	"/sys/class/dmi/id/product_version",
}

var (
	returnTrue  = &types.BoolValue{V: true}
	returnFalse = &types.BoolValue{V: false}
)

func init() {
	simple.ModuleRegister(ModuleName, "is_virtual", &simple.Scaffold{
		T: types.NewType("func() bool"),
		F: IsVirtual,
	})
}

// IsVirtual is a simple function that executes two types of checks: first, we
// check whether we're running on Linux. If that's the case, we run checks
// related with the presence of virtualization platforms. If any of those checks
// returns true, then so does this function. Otherwise, we assume that we're not
// in a virtualized environment.
func IsVirtual(ctx context.Context, input []types.Value) (types.Value, error) {
	// If we implement detection for OS other than Linux, this logic will have
	// to change
	opersys := runtime.GOOS
	if opersys != "linux" {
		return nil, errors.New("we're not running on Linux, exiting")
	}

	// If we're on x86, we can make use of a simple check to detect virt envs
	cpuInfo, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		if strings.Contains(string(cpuInfo), "hypervisor") {
			return returnTrue, nil
		}
	}

	// We make use of systemd's work for detecting virtualization platforms, and
	// check if any of the keys of virtualizationVendorMap is present on any of
	// the DMI files present on dmiFilesSlice. If that's the case, then we
	// return this func as true.
	// https://github.com/systemd/systemd/blob/main/src/basic/virt.c
	for _, dmiFile := range dmiFilesSlice {
		dmiFileContent, err := os.ReadFile(dmiFile)
		if err == nil {
			for key := range virtualizationVendorMap {
				if strings.Contains(string(dmiFileContent), key) {
					return returnTrue, nil
				}
			}
		}
	}

	return returnFalse, nil
}
