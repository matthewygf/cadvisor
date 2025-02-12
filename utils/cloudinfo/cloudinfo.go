// Copyright 2015 Google Inc. All Rights Reserved.
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

// Get information about the cloud provider (if any) cAdvisor is running on.

package cloudinfo

import (
	info "github.com/matthewygf/cadvisor/info/v1"
	"k8s.io/klog"
)

type CloudInfo interface {
	GetCloudProvider() info.CloudProvider
	GetInstanceType() info.InstanceType
	GetInstanceID() info.InstanceID
}

// CloudProvider is an abstraction for providing cloud-specific information.
type CloudProvider interface {
	// IsActiveProvider determines whether this is the cloud provider operating
	// this instance.
	IsActiveProvider() bool
	// GetInstanceType gets the type of instance this process is running on.
	// The behavior is undefined if this is not the active provider.
	GetInstanceType() info.InstanceType
	// GetInstanceType gets the ID of the instance this process is running on.
	// The behavior is undefined if this is not the active provider.
	GetInstanceID() info.InstanceID
}

var providers = map[info.CloudProvider]CloudProvider{}

// RegisterCloudProvider registers the given cloud provider
func RegisterCloudProvider(name info.CloudProvider, provider CloudProvider) {
	if _, alreadyRegistered := providers[name]; alreadyRegistered {
		klog.Warningf("Duplicate registration of CloudProvider %s", name)
	}
	providers[name] = provider
}

type realCloudInfo struct {
	cloudProvider info.CloudProvider
	instanceType  info.InstanceType
	instanceID    info.InstanceID
}

func NewRealCloudInfo() CloudInfo {
	for name, provider := range providers {
		if provider.IsActiveProvider() {
			return &realCloudInfo{
				cloudProvider: name,
				instanceType:  provider.GetInstanceType(),
				instanceID:    provider.GetInstanceID(),
			}
		}
	}

	// No registered active provider.
	return &realCloudInfo{
		cloudProvider: info.UnknownProvider,
		instanceType:  info.UnknownInstance,
		instanceID:    info.UnNamedInstance,
	}
}

func (self *realCloudInfo) GetCloudProvider() info.CloudProvider {
	return self.cloudProvider
}

func (self *realCloudInfo) GetInstanceType() info.InstanceType {
	return self.instanceType
}

func (self *realCloudInfo) GetInstanceID() info.InstanceID {
	return self.instanceID
}
