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

package raw

import (
	"flag"
	"fmt"
	"strings"

	"github.com/matthewygf/cadvisor/container"
	"github.com/matthewygf/cadvisor/container/common"
	"github.com/matthewygf/cadvisor/container/libcontainer"
	"github.com/matthewygf/cadvisor/fs"
	info "github.com/matthewygf/cadvisor/info/v1"
	watch "github.com/matthewygf/cadvisor/watcher"

	"k8s.io/klog"
)

var dockerOnly = flag.Bool("docker_only", false, "Only report docker containers in addition to root stats")
var disableRootCgroupStats = flag.Bool("disable_root_cgroup_stats", false, "Disable collecting root Cgroup stats")

type rawFactory struct {
	// Factory for machine information.
	machineInfoFactory info.MachineInfoFactory

	// Information about the cgroup subsystems.
	cgroupSubsystems *libcontainer.CgroupSubsystems

	// Information about mounted filesystems.
	fsInfo fs.FsInfo

	// Watcher for inotify events.
	watcher *common.InotifyWatcher

	// List of metrics to be included.
	includedMetrics map[container.MetricKind]struct{}

	// List of raw container cgroup path prefix whitelist.
	rawPrefixWhiteList []string
}

func (self *rawFactory) String() string {
	return "raw"
}

func (self *rawFactory) NewContainerHandler(name string, inHostNamespace bool) (container.ContainerHandler, error) {
	rootFs := "/"
	if !inHostNamespace {
		rootFs = "/rootfs"
	}
	return newRawContainerHandler(name, self.cgroupSubsystems, self.machineInfoFactory, self.fsInfo, self.watcher, rootFs, self.includedMetrics)
}

// The raw factory can handle any container. If --docker_only is set to true, non-docker containers are ignored except for "/" and those whitelisted by raw_cgroup_prefix_whitelist flag.
func (self *rawFactory) CanHandleAndAccept(name string) (bool, bool, error) {
	if name == "/" {
		return true, true, nil
	}
	if *dockerOnly && self.rawPrefixWhiteList[0] == "" {
		return true, false, nil
	}
	for _, prefix := range self.rawPrefixWhiteList {
		if strings.HasPrefix(name, prefix) {
			return true, true, nil
		}
	}
	return true, false, nil
}

func (self *rawFactory) DebugInfo() map[string][]string {
	return common.DebugInfo(self.watcher.GetWatches())
}

func Register(machineInfoFactory info.MachineInfoFactory, fsInfo fs.FsInfo, includedMetrics map[container.MetricKind]struct{}, rawPrefixWhiteList []string) error {
	cgroupSubsystems, err := libcontainer.GetCgroupSubsystems(includedMetrics)
	if err != nil {
		return fmt.Errorf("failed to get cgroup subsystems: %v", err)
	}
	if len(cgroupSubsystems.Mounts) == 0 {
		return fmt.Errorf("failed to find supported cgroup mounts for the raw factory")
	}

	watcher, err := common.NewInotifyWatcher()
	if err != nil {
		return err
	}

	klog.V(1).Infof("Registering Raw factory")
	factory := &rawFactory{
		machineInfoFactory: machineInfoFactory,
		fsInfo:             fsInfo,
		cgroupSubsystems:   &cgroupSubsystems,
		watcher:            watcher,
		includedMetrics:    includedMetrics,
		rawPrefixWhiteList: rawPrefixWhiteList,
	}
	container.RegisterContainerHandlerFactory(factory, []watch.ContainerWatchSource{watch.Raw})
	return nil
}
