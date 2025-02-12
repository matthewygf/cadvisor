// Copyright 2016 Google Inc. All Rights Reserved.
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

package rkt

import (
	"fmt"

	"github.com/matthewygf/cadvisor/container"
	"github.com/matthewygf/cadvisor/container/libcontainer"
	"github.com/matthewygf/cadvisor/fs"
	info "github.com/matthewygf/cadvisor/info/v1"
	"github.com/matthewygf/cadvisor/watcher"

	"k8s.io/klog"
)

const RktNamespace = "rkt"

type rktFactory struct {
	machineInfoFactory info.MachineInfoFactory

	cgroupSubsystems *libcontainer.CgroupSubsystems

	fsInfo fs.FsInfo

	includedMetrics container.MetricSet

	rktPath string
}

func (self *rktFactory) String() string {
	return "rkt"
}

func (self *rktFactory) NewContainerHandler(name string, inHostNamespace bool) (container.ContainerHandler, error) {
	client, err := Client()
	if err != nil {
		return nil, err
	}

	rootFs := "/"
	if !inHostNamespace {
		rootFs = "/rootfs"
	}
	return newRktContainerHandler(name, client, self.rktPath, self.cgroupSubsystems, self.machineInfoFactory, self.fsInfo, rootFs, self.includedMetrics)
}

func (self *rktFactory) CanHandleAndAccept(name string) (bool, bool, error) {
	accept, err := verifyPod(name)

	return accept, accept, err
}

func (self *rktFactory) DebugInfo() map[string][]string {
	return map[string][]string{}
}

func Register(machineInfoFactory info.MachineInfoFactory, fsInfo fs.FsInfo, includedMetrics container.MetricSet) error {
	_, err := Client()
	if err != nil {
		return fmt.Errorf("unable to communicate with Rkt api service: %v", err)
	}

	rktPath, err := RktPath()
	if err != nil {
		return fmt.Errorf("unable to get the RktPath variable %v", err)
	}

	cgroupSubsystems, err := libcontainer.GetCgroupSubsystems(includedMetrics)
	if err != nil {
		return fmt.Errorf("failed to get cgroup subsystems: %v", err)
	}
	if len(cgroupSubsystems.Mounts) == 0 {
		return fmt.Errorf("failed to find supported cgroup mounts for the raw factory")
	}

	klog.V(1).Infof("Registering Rkt factory")
	factory := &rktFactory{
		machineInfoFactory: machineInfoFactory,
		fsInfo:             fsInfo,
		cgroupSubsystems:   &cgroupSubsystems,
		includedMetrics:    includedMetrics,
		rktPath:            rktPath,
	}
	container.RegisterContainerHandlerFactory(factory, []watcher.ContainerWatchSource{watcher.Rkt})
	return nil
}
