// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package charts

import (
	"embed"
	_ "embed"
	"path/filepath"
)

var (
	// ChartMachineControllerManagerSeed is the Helm chart for the machine-controller-manager/seed chart.
	//go:embed machine-controller-manager/seed
	ChartMachineControllerManagerSeed embed.FS
	// ChartPathMachineControllerManagerSeed is the path to the machine-controller-manager/seed chart.
	ChartPathMachineControllerManagerSeed = filepath.Join("machine-controller-manager", "seed")

	// ChartMachineControllerManagerShoot is the Helm chart for the machine-controller-manager/shoot chart.
	//go:embed machine-controller-manager/shoot
	ChartMachineControllerManagerShoot embed.FS
	// ChartPathMachineControllerManagerShoot is the path to the machine-controller-manager/shoot chart.
	ChartPathMachineControllerManagerShoot = filepath.Join("machine-controller-manager", "shoot")

	// ChartShootStorageClasses is the Helm chart for the shoot-storageclasses chart.
	//go:embed shoot-storageclasses
	ChartShootStorageClasses embed.FS
	// ChartPathShootStorageClasses is the path to the shoot-storageclasses chart.
	ChartPathShootStorageClasses = filepath.Join("shoot-storageclasses")

	// ChartShootSystemComponents is the Helm chart for the shoot-system-components chart.
	//go:embed shoot-system-components
	ChartShootSystemComponents embed.FS
	// ChartPathShootSystemComponents is the path to the shoot-system-components chart.
	ChartPathShootSystemComponents = filepath.Join("shoot-system-components")
)
