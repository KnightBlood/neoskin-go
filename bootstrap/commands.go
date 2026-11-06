package bootstrap

import (
	"github.com/goravel/framework/contracts/console"

	"neoskin/app/console/commands"
)

func Commands() []console.Command {
	return []console.Command{
		&commands.InstallDatabase{},
		&commands.Update{},
		&commands.BsInstall{},
		&commands.OptionsCache{},
		&commands.PluginEnable{},
		&commands.PluginDisable{},
		&commands.SaltRandom{},
	}
}
