// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package machine

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/parsers/operatingsystem"
	"github.com/matthewygf/cadvisor/fs"
	info "github.com/matthewygf/cadvisor/info/v1"
	"github.com/matthewygf/cadvisor/utils/cloudinfo"
	"github.com/matthewygf/cadvisor/utils/sysfs"
	"github.com/matthewygf/cadvisor/utils/sysinfo"

	"k8s.io/klog"

	"golang.org/x/sys/unix"
)

const hugepagesDirectory = "/sys/kernel/mm/hugepages/"

var machineIdFilePath = flag.String("machine_id_file", "/etc/machine-id,/var/lib/dbus/machine-id", "Comma-separated list of files to check for machine-id. Use the first one that exists.")
var bootIdFilePath = flag.String("boot_id_file", "/proc/sys/kernel/random/boot_id", "Comma-separated list of files to check for boot-id. Use the first one that exists.")

func getInfoFromFiles(filePaths string) string {
	if len(filePaths) == 0 {
		return ""
	}
	for _, file := range strings.Split(filePaths, ",") {
		id, err := ioutil.ReadFile(file)
		if err == nil {
			return strings.TrimSpace(string(id))
		}
	}
	klog.Warningf("Couldn't collect info from any of the files in %q", filePaths)
	return ""
}

// GetHugePagesInfo returns information about pre-allocated huge pages
func GetHugePagesInfo() ([]info.HugePagesInfo, error) {
	var hugePagesInfo []info.HugePagesInfo
	files, err := ioutil.ReadDir(hugepagesDirectory)
	if err != nil {
		// treat as non-fatal since kernels and machine can be
		// configured to disable hugepage support
		return hugePagesInfo, nil
	}
	for _, st := range files {
		nameArray := strings.Split(st.Name(), "-")
		pageSizeArray := strings.Split(nameArray[1], "kB")
		pageSize, err := strconv.ParseUint(string(pageSizeArray[0]), 10, 64)
		if err != nil {
			return hugePagesInfo, err
		}

		numFile := hugepagesDirectory + st.Name() + "/nr_hugepages"
		val, err := ioutil.ReadFile(numFile)
		if err != nil {
			return hugePagesInfo, err
		}
		var numPages uint64
		// we use sscanf as the file as a new-line that trips up ParseUint
		// it returns the number of tokens successfully parsed, so if
		// n != 1, it means we were unable to parse a number from the file
		n, err := fmt.Sscanf(string(val), "%d", &numPages)
		if err != nil || n != 1 {
			return hugePagesInfo, fmt.Errorf("could not parse file %v contents %q", numFile, string(val))
		}

		hugePagesInfo = append(hugePagesInfo, info.HugePagesInfo{
			NumPages: numPages,
			PageSize: pageSize,
		})
	}
	return hugePagesInfo, nil
}

func Info(sysFs sysfs.SysFs, fsInfo fs.FsInfo, inHostNamespace bool) (*info.MachineInfo, error) {
	rootFs := "/"
	if !inHostNamespace {
		rootFs = "/rootfs"
	}

	cpuinfo, err := ioutil.ReadFile(filepath.Join(rootFs, "/proc/cpuinfo"))
	if err != nil {
		return nil, err
	}
	clockSpeed, err := GetClockSpeed(cpuinfo)
	if err != nil {
		return nil, err
	}

	memoryCapacity, err := GetMachineMemoryCapacity()
	if err != nil {
		return nil, err
	}

	hugePagesInfo, err := GetHugePagesInfo()
	if err != nil {
		return nil, err
	}

	filesystems, err := fsInfo.GetGlobalFsInfo()
	if err != nil {
		klog.Errorf("Failed to get global filesystem information: %v", err)
	}

	diskMap, err := sysinfo.GetBlockDeviceInfo(sysFs)
	if err != nil {
		klog.Errorf("Failed to get disk map: %v", err)
	}

	netDevices, err := sysinfo.GetNetworkDevices(sysFs)
	if err != nil {
		klog.Errorf("Failed to get network devices: %v", err)
	}

	topology, numCores, err := GetTopology(sysFs, string(cpuinfo))
	if err != nil {
		klog.Errorf("Failed to get topology information: %v", err)
	}

	systemUUID, err := sysinfo.GetSystemUUID(sysFs)
	if err != nil {
		klog.Errorf("Failed to get system UUID: %v", err)
	}

	realCloudInfo := cloudinfo.NewRealCloudInfo()
	cloudProvider := realCloudInfo.GetCloudProvider()
	instanceType := realCloudInfo.GetInstanceType()
	instanceID := realCloudInfo.GetInstanceID()

	machineInfo := &info.MachineInfo{
		NumCores:       numCores,
		CpuFrequency:   clockSpeed,
		MemoryCapacity: memoryCapacity,
		HugePages:      hugePagesInfo,
		DiskMap:        diskMap,
		NetworkDevices: netDevices,
		Topology:       topology,
		MachineID:      getInfoFromFiles(filepath.Join(rootFs, *machineIdFilePath)),
		SystemUUID:     systemUUID,
		BootID:         getInfoFromFiles(filepath.Join(rootFs, *bootIdFilePath)),
		CloudProvider:  cloudProvider,
		InstanceType:   instanceType,
		InstanceID:     instanceID,
	}

	for i := range filesystems {
		fs := filesystems[i]
		inodes := uint64(0)
		if fs.Inodes != nil {
			inodes = *fs.Inodes
		}
		machineInfo.Filesystems = append(machineInfo.Filesystems, info.FsInfo{Device: fs.Device, DeviceMajor: uint64(fs.Major), DeviceMinor: uint64(fs.Minor), Type: fs.Type.String(), Capacity: fs.Capacity, Inodes: inodes, HasInodes: fs.Inodes != nil})
	}

	return machineInfo, nil
}

func ContainerOsVersion() string {
	os, err := operatingsystem.GetOperatingSystem()
	if err != nil {
		os = "Unknown"
	}
	return os
}

func KernelVersion() string {
	uname := &unix.Utsname{}

	if err := unix.Uname(uname); err != nil {
		return "Unknown"
	}

	return string(uname.Release[:bytes.IndexByte(uname.Release[:], 0)])
}
