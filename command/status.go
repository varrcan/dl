package command

import (
	"context"
	"fmt"
	"sort"

	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/local-deploy/dl/utils"
	"github.com/local-deploy/dl/utils/docker"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func statusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show dl status",
		Long:  `List of containers started by dl.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runStatus()
			if err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}

func runStatus() error {
	ctx := context.Background()

	cli, err := docker.NewClient()
	if err != nil {
		pterm.FgRed.Printfln("Failed to connect to socket")
		return err
	}

	pterm.DefaultBasicText.Print("# ")
	if cli.IsServiceRunning(ctx) {
		green := pterm.NewStyle(pterm.FgLightGreen, pterm.BgDefault, pterm.Bold)
		green.Println("dl is running")
	} else {
		red := pterm.NewStyle(pterm.FgLightRed, pterm.BgDefault, pterm.Bold)
		red.Println("dl is not running")

		return nil
	}

	services, err := getServices(ctx, cli)
	if err != nil {
		return err
	}

	projects, err := getProjects(ctx, cli)
	if err != nil {
		return err
	}

	err = render(cli, "services", services)
	if err != nil {
		return err
	}

	err = render(cli, "projects", projects)
	if err != nil {
		return err
	}

	return nil
}

func getServices(ctx context.Context, cli *docker.Client) ([]docker.ContainerSummary, error) {
	containerFilter := filters.NewArgs(
		filters.Arg("label", fmt.Sprintf("%s=%s", api.ProjectLabel, "dl-services")),
	)
	containers, _ := cli.DockerCli.Client().ContainerList(ctx, container.ListOptions{Filters: containerFilter, All: true})

	return calculate(ctx, cli, containers)
}

func getProjects(ctx context.Context, cli *docker.Client) ([]docker.ContainerSummary, error) {
	containerFilter := filters.NewArgs(
		filters.Arg("label", fmt.Sprintf("%s=%s", api.WorkingDirLabel, utils.TemplateDir())),
	)
	containers, _ := cli.DockerCli.Client().ContainerList(ctx, container.ListOptions{Filters: containerFilter, All: true})

	return calculate(ctx, cli, containers)
}

func calculate(ctx context.Context, cli *docker.Client, containers []types.Container) ([]docker.ContainerSummary, error) {
	summary := make([]docker.ContainerSummary, len(containers))
	eg, ctx := errgroup.WithContext(ctx)
	for i, c := range containers {
		i, con := i, c
		eg.Go(func() error {
			var publishers []docker.PortPublisher
			sort.Slice(con.Ports, func(i, j int) bool {
				return con.Ports[i].PrivatePort < con.Ports[j].PrivatePort
			})
			for _, port := range con.Ports {
				publishers = append(publishers, docker.PortPublisher{
					URL:           port.IP,
					TargetPort:    int(port.PrivatePort),
					PublishedPort: int(port.PublicPort),
					Protocol:      port.Type,
				})
			}

			inspect, err := cli.DockerCli.Client().ContainerInspect(ctx, con.ID)
			if err != nil {
				return err
			}

			var (
				health   string
				exitCode int
			)
			if inspect.State != nil {
				switch inspect.State.Status {
				case "running":
					if inspect.State.Health != nil {
						health = inspect.State.Health.Status
					}
				case "exited", "dead":
					exitCode = inspect.State.ExitCode
				}
			}

			summary[i] = docker.ContainerSummary{
				ID:         con.ID,
				Name:       docker.GetCanonicalContainerName(con),
				State:      con.State,
				Health:     health,
				ExitCode:   exitCode,
				Publishers: publishers,
			}
			return nil
		})
	}
	return summary, eg.Wait()
}

func render(cli *docker.Client, title string, containers []docker.ContainerSummary) error {
	if len(containers) == 0 {
		return nil
	}

	pterm.Println()
	pterm.DefaultBasicText.Print("## ")
	green := pterm.NewStyle(pterm.FgLightYellow, pterm.BgDefault, pterm.Bold)
	green.Println(title)

	data := make([][]string, len(containers)+1)
	data[0] = []string{"ID", "Name", "State", "Ports"}
	for _, con := range containers {
		status := con.State
		if status == "running" && con.Health != "" {
			status = fmt.Sprintf("%s (%s)", con.State, con.Health)
		} else if status == "exited" || status == "dead" {
			status = fmt.Sprintf("%s (%d)", con.State, con.ExitCode)
		}
		con := []string{con.ID[:12], con.Name, status, cli.DisplayablePorts(con)}
		data = append(data, con)
	}

	err := pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	if err != nil {
		return err
	}

	return err
}
