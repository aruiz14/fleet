package main

import (
	"github.com/rancher/fleet/modules/cli/pkg/command"
	"github.com/rancher/fleet/pkg/fleetcontroller"
	"github.com/rancher/fleet/pkg/version"
	"github.com/spf13/cobra"
)

var (
	debugConfig command.DebugConfig
)

type FleetManager struct {
	Kubeconfig string `usage:"Kubeconfig file"`
	Namespace  string `usage:"namespace to watch" default:"fleet-system" env:"NAMESPACE"`
}

func (f *FleetManager) Run(cmd *cobra.Command, args []string) error {
	debugConfig.MustSetupDebug()
	if err := fleetcontroller.Start(cmd.Context(), f.Namespace, f.Kubeconfig); err != nil {
		return err
	}

	<-cmd.Context().Done()
	return nil
}

func main() {
	cmd := command.Command(&FleetManager{}, cobra.Command{
		Version: version.FriendlyVersion(),
	})
	cmd = command.AddDebug(cmd, &debugConfig)
	command.Main(cmd)
}
