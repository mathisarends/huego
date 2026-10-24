package cmd

import (
	"context"
	"fmt"

	"github.com/mathisarends/huego/internal/hue"
	"github.com/spf13/cobra"
)

type groupTarget interface {
	controlTarget
	ListWithState(ctx context.Context) ([]hue.Group, error)
	GetWithState(ctx context.Context, id string) (hue.Group, error)
}

// newGroupCommand builds the room/zone command: list and get plus the
// shared control vocabulary, all acting through the grouped_light service.
func newGroupCommand(kind, plural string, target func(*hue.Client) groupTarget) *cobra.Command {
	parent := &cobra.Command{
		Use:     kind,
		Aliases: []string{plural},
		Short:   fmt.Sprintf("Inspect and control %s", plural),
	}

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   fmt.Sprintf("List %s with their aggregate state and brightness", plural),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			groups, err := target(client).ListWithState(cmd.Context())
			if err != nil {
				return err
			}
			return writeList(cmd, mapViews(groups, newGroupView), groupListing)
		},
	}
	addNamesFlag(listCmd)

	getCmd := &cobra.Command{
		Use:   "get <target>",
		Short: fmt.Sprintf("Show one %s's state and the lights in it", kind),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newClient()
			if err != nil {
				return err
			}
			id, err := target(client).ResolveTarget(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			group, err := target(client).GetWithState(cmd.Context(), id)
			if err != nil {
				return err
			}
			view := newGroupView(group)
			return writeDetails(cmd, view, view.fields())
		},
	}

	parent.AddCommand(listCmd, getCmd)
	addControlCommands(parent, func(c *hue.Client) controlTarget { return target(c) })
	return parent
}

func findByID[T interface{ GetID() string }](items []T, id, resourceType string) (T, error) {
	for _, item := range items {
		if item.GetID() == id {
			return item, nil
		}
	}
	var zero T
	return zero, &hue.ResourceNotFoundError{Message: fmt.Sprintf("no %s with ID %s", resourceType, id)}
}

func init() {
	rootCmd.AddCommand(
		newGroupCommand("room", "rooms", func(c *hue.Client) groupTarget { return c.Rooms }),
		newGroupCommand("zone", "zones", func(c *hue.Client) groupTarget { return c.Zones }),
	)
}
