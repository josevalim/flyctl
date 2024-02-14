package postgres

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"github.com/superfly/fly-go/api"
	"github.com/superfly/flyctl/flypg"
	"github.com/superfly/flyctl/internal/command"
	mach "github.com/superfly/flyctl/internal/machine"
)

func New() *cobra.Command {
	const (
		short = `Manage Postgres clusters.`

		long = short + "\n"
	)

	cmd := command.New("postgres", short, long, nil)

	cmd.Aliases = []string{"pg"}

	cmd.AddCommand(
		newAttach(),
		newConfig(),
		newConnect(),
		newCreate(),
		newDb(),
		newDetach(),
		newList(),
		newRestart(),
		newUsers(),
		newFailover(),
		newAddFlycast(),
		newImport(),
		newEvents(),
		newBarman(),
	)

	return cmd
}

func hasRequiredVersionOnMachines(machines []*api.Machine, cluster, flex, standalone string) error {
	_, dev := os.LookupEnv("FLY_DEV")
	if dev {
		return nil
	}

	for _, machine := range machines {
		// Validate image version to ensure it's compatible with this feature.
		if machine.ImageVersion() == "" || machine.ImageVersion() == "unknown" {
			return fmt.Errorf("command is not compatible with this image")
		}

		imageVersionStr := machine.ImageVersion()[1:]

		imageVersion, err := version.NewVersion(imageVersionStr)
		if err != nil {
			return err
		}

		// Specify compatible versions per repo.
		requiredVersion := &version.Version{}
		if machine.ImageRepository() == "flyio/postgres-standalone" {
			requiredVersion, err = version.NewVersion(standalone)
			if err != nil {
				return err
			}
		}
		if machine.ImageRepository() == "flyio/postgres" {
			requiredVersion, err = version.NewVersion(cluster)
			if err != nil {
				return err
			}
		}

		if machine.ImageRepository() == "flyio/postgres-timescaledb" {
			requiredVersion, err = version.NewVersion(cluster)
			if err != nil {
				return err
			}
		}

		if IsFlex(machine) {
			requiredVersion, err = version.NewVersion(flex)
			if err != nil {
				return err
			}
		}

		if requiredVersion == nil {
			return fmt.Errorf("unable to resolve image version")
		}

		if imageVersion.LessThan(requiredVersion) {
			return fmt.Errorf(
				"%s is running an incompatible image version. (Current: %s, Required: >= %s)\n"+
					"Please run 'flyctl pg update' to update to the latest available version",
				machine.ID, imageVersion, requiredVersion.String())
		}

	}
	return nil
}

func IsFlex(machine *api.Machine) bool {
	switch {
	case machine == nil || len(machine.ImageRef.Labels) == 0:
		return false
	case machine.ImageRef.Labels["fly.pg-manager"] == "repmgr":
		return true
	default:
		return false
	}
}

func machinesNodeRoles(ctx context.Context, machines []*api.Machine) (leader *api.Machine, replicas []*api.Machine) {
	for _, machine := range machines {
		role := machineRole(machine)

		switch role {
		case "leader", "primary":
			leader = machine
		case "replica", "standby":
			replicas = append(replicas, machine)
		default:
			replicas = append(replicas, machine)
		}
	}
	return leader, replicas
}

func machineRole(machine *api.Machine) (role string) {
	role = "unknown"

	for _, check := range machine.Checks {
		if check.Name == "role" {
			if check.Status == api.Passing {
				role = check.Output
			} else {
				role = "error"
			}
			break
		}
	}
	return role
}

func isLeader(machine *api.Machine) bool {
	return machineRole(machine) == "leader" || machineRole(machine) == "primary"
}

func pickLeader(ctx context.Context, machines []*api.Machine) (*api.Machine, error) {
	for _, machine := range machines {
		if isLeader(machine) {
			return machine, nil
		}
	}
	return nil, fmt.Errorf("no active leader found")
}

func UnregisterMember(ctx context.Context, app *api.AppCompact, machine *api.Machine) error {
	machines, err := mach.ListActive(ctx)
	if err != nil {
		return err
	}

	leader, err := pickLeader(ctx, machines)
	if err != nil {
		return err
	}

	cmd, err := flypg.NewCommand(ctx, app)
	if err != nil {
		return err
	}

	if err := cmd.UnregisterMember(ctx, leader.PrivateIP, machine.PrivateIP); err != nil {
		return err
	}

	return nil
}
